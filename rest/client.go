package rest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	defaultClientPort int = 3000
	maxClientPort         = 65535
)

// Client provides an interface for interacting with a remote amboy
// Service.
type Client struct {
	host   string
	prefix string
	port   int
	client *http.Client
}

// NewClient takes host, port, and URI prefix information and
// constructs a new Client.
func NewClient(host string, port int, prefix string) (*Client, error) {
	c := &Client{client: &http.Client{}}

	return c.initClient(host, port, prefix)
}

// NewClientFromExisting takes an existing http.Client object and
// produces a new Client object.
func NewClientFromExisting(client *http.Client, host string, port int, prefix string) (*Client, error) {
	if client == nil {
		return nil, errors.New("must use a non-nil existing client")
	}

	c := &Client{client: client}

	return c.initClient(host, port, prefix)
}

// Copy takes an existing Client object and returns a new client
// object with the same settings that uses a *new* http.Client.
func (c *Client) Copy() *Client {
	new := &Client{}
	*new = *c
	new.client = &http.Client{}

	return new
}

func (c *Client) initClient(host string, port int, prefix string) (*Client, error) {
	if err := c.SetHost(host); err != nil {
		return nil, err
	}

	if err := c.SetPort(port); err != nil {
		return nil, err
	}

	if err := c.SetPrefix(prefix); err != nil {
		return nil, err
	}

	return c, nil
}

////////////////////////////////////////////////////////////////////////
//
// Configuration Interface
//
////////////////////////////////////////////////////////////////////////

// Client returns a pointer to embedded http.Client object.
func (c *Client) Client() *http.Client {
	return c.client
}

// SetHost allows callers to change the hostname (including leading
// "http(s)") for the Client. Returns an error if the specified host
// does not start with "http".
func (c *Client) SetHost(h string) error {
	if !strings.HasPrefix(h, "http") {
		return errors.Errorf("host '%s' is malformed. must start with 'http'", h)
	}

	if strings.HasSuffix(h, "/") {
		h = h[:len(h)-1]
	}

	c.host = h

	return nil
}

// Host returns the current host.
func (c *Client) Host() string {
	return c.host
}

// SetPort allows callers to change the port used for the client. If
// the port is invalid, returns an error and sets the port to the
// default value. (3000)
func (c *Client) SetPort(p int) error {
	if p <= 0 || p >= maxClientPort {
		c.port = defaultClientPort
		return errors.Errorf("cannot set the port to %d, using %d instead", p, defaultClientPort)
	}

	c.port = p
	return nil
}

// Port returns the current port value for the Client.
func (c *Client) Port() int {
	return c.port
}

// SetPrefix allows callers to modify the prefix, for this client,
func (c *Client) SetPrefix(p string) error {
	c.prefix = strings.Trim(p, "/")
	return nil
}

// Prefix accesses the prefix for the client, The prefix is the part
// of the URI between the end-point and the hostname, of the API.
func (c *Client) Prefix() string {
	return c.prefix
}

func (c *Client) getURL(endpoint string) string {
	var url []string

	if strings.HasPrefix(endpoint, c.host) {
		return endpoint
	}

	if c.port == 80 || c.port == 0 {
		url = append(url, c.host)
	} else {
		url = append(url, fmt.Sprintf("%s:%d", c.host, c.port))
	}

	if c.prefix != "" {
		url = append(url, c.prefix)
	}

	if endpoint = strings.Trim(endpoint, "/"); endpoint != "" {
		url = append(url, endpoint)
	}

	return strings.Join(url, "/")
}

func (c *Client) makeRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.getURL(url), body)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	req = req.WithContext(ctx)
	grip.Debug(message.Fields{
		"method":   method,
		"url":      url,
		"host":     c.host,
		"nil_body": body == nil,
	})
	return req, nil
}

////////////////////////////////////////////////////////////////////////
//
// Public Operations that Interact with the Service
//
////////////////////////////////////////////////////////////////////////

func (c *Client) GetStatus(ctx context.Context) (*StatusResponse, error) {
	out := &StatusResponse{}

	req, err := c.makeRequest(ctx, http.MethodGet, "/v1/status", nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading rstatus result")
	}

	return out, nil
}

///////////////////////////////////
//
// Simple Log Example Handler

func (c *Client) WriteSimpleLog(ctx context.Context, logID, data string, increment int) (*SimpleLogInjestionResponse, error) {
	payload, err := json.Marshal(simpleLogRequest{
		Time:      time.Now(),
		Increment: increment,
		Content:   data,
	})
	if err != nil {
		return nil, errors.Wrap(err, "problem converting json")
	}

	r, err := c.makeRequest(ctx, http.MethodPost, fmt.Sprintf("/v1/simple_log/%s", logID), bytes.NewBuffer(payload))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(r)
	if err != nil {
		grip.Warning(err)
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &SimpleLogInjestionResponse{}

	if err := gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem parsing request")
	}

	return out, nil
}

func (c *Client) GetSimpleLog(ctx context.Context, logID string) (*SimpleLogContentResponse, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/simple_log/%s", logID), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &SimpleLogContentResponse{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading simple log result")
	}

	return out, nil
}

func (c *Client) GetSimpleLogText(ctx context.Context, logID string) ([]string, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/simple_log/%s/text", logID), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	var out []string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}

	return out, errors.Wrap(scanner.Err(), "problem reading output")
}

///////////////////////////////////
//
// System Events/Logging

func (c *Client) GetSystemEvents(ctx context.Context, level string, limit int) (*SystemEventsResponse, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/admin/status/events/%s?limit=%d", level, limit), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &SystemEventsResponse{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system status result")
	}

	return out, nil
}

func (c *Client) GetSystemEvent(ctx context.Context, id string) (*SystemEventResponse, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, "/v1/admin/status/events/"+id, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}

	defer resp.Body.Close()

	out := &SystemEventResponse{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system status result")
	}

	return out, nil
}

func (c *Client) AcknowledgeSystemEvent(ctx context.Context, id string) (*SystemEventResponse, error) {
	req, err := c.makeRequest(ctx, http.MethodPost, fmt.Sprintf("/v1/admin/status/events/%s/acknowledge", id), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}

	defer resp.Body.Close()

	out := &SystemEventResponse{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system status result")
	}

	return out, nil
}

///////////////////////////////////
//
// System Information

func (c *Client) SendSystemInfo(ctx context.Context, info *message.SystemInfo) (*SystemInfoReceivedResponse, error) {
	payload, err := json.Marshal(info)
	if err != nil {
		return nil, errors.Wrap(err, "problem converting json")
	}

	req, err := c.makeRequest(ctx, http.MethodPost, "/v1/system_info", bytes.NewBuffer(payload))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &SystemInfoReceivedResponse{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system info result")
	}

	return out, nil
}

func (c *Client) GetSystemInformation(ctx context.Context, host string, start, end time.Time, limit int) ([]*message.SystemInfo, error) {
	url := c.getURL(fmt.Sprintf("/v1/system_info/host/%s?limit=%d", host, limit))
	if !start.IsZero() {
		url += fmt.Sprintf("&start=%s", start.Format(time.RFC3339))
	}

	if !end.IsZero() {
		url += fmt.Sprintf("&end=%s", end.Format(time.RFC3339))
	}

	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &SystemInformationResponse{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system status result")
	}

	if out.Error != "" {
		return nil, errors.Errorf("encountered problem server-side: %s", out.Error)
	}

	return out.Data, nil
}

///////////////////////////////////
//
// Admin

func (c *Client) EnableFeatureFlag(ctx context.Context, name string) (bool, error) {
	url := c.getURL(fmt.Sprintf("/v1/admin/service/flag/%s/enabled", name))

	req, err := c.makeRequest(ctx, http.MethodPost, url, nil)
	if err != nil {
		return false, errors.Wrap(err, "problem building request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &serviceFlagResponse{}

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return false, errors.Wrap(err, "problem reading feature flag result")
	}

	if out.Error != "" {
		return out.State, errors.Errorf("encountered problem server-side: %s", out.Error)
	}
	return out.State, nil
}

func (c *Client) DisableFeatureFlag(ctx context.Context, name string) (bool, error) {
	url := c.getURL(fmt.Sprintf("/v1/admin/service/flag/%s/disabled", name))

	req, err := c.makeRequest(ctx, http.MethodPost, url, nil)
	if err != nil {
		return false, errors.Wrap(err, "problem building request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	out := &serviceFlagResponse{}

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return false, errors.Wrap(err, "problem reading feature flag result")
	}

	if out.Error != "" {
		return out.State, errors.Errorf("encountered problem server-side: %s", out.Error)
	}
	return out.State, nil
}

func (c *Client) GetAuthKey(ctx context.Context, username, password string) (string, error) {
	url := c.getURL("/v1/admin/users/key")

	creds := &userCredentials{Username: username, Password: password}
	payload, err := json.Marshal(creds)
	if err != nil {
		return "", errors.Wrap(err, "problem building payload")
	}

	req, err := c.makeRequest(ctx, http.MethodGet, url, bytes.NewBuffer(payload))
	if err != nil {
		return "", errors.Wrap(err, "problem building request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		srverr := gimlet.ErrorResponse{}
		if err := gimlet.GetJSON(resp.Body, &srverr); err != nil {
			return "", errors.Wrap(err, "problem parsing error message")
		}

		return "", srverr
	}

	out := &userAPIKeyResponse{}
	if err := gimlet.GetJSON(resp.Body, out); err != nil {
		return "", errors.Wrap(err, "problem parsing response")
	}

	if username != out.Username {
		return "", errors.New("service error: mismatched usernames")
	}

	return out.Key, nil
}

func (c *Client) GetUserCertificate(ctx context.Context, username, password string) (string, error) {
	url := c.getURL("/v1/admin/users/cert")

	creds := &userCredentials{Username: username, Password: password}
	payload, err := json.Marshal(creds)
	if err != nil {
		return "", errors.Wrap(err, "problem building payload")
	}

	req, err := c.makeRequest(ctx, http.MethodGet, url, bytes.NewBuffer(payload))
	if err != nil {
		return "", errors.Wrap(err, "problem building request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		srverr := gimlet.ErrorResponse{}
		if err := gimlet.GetJSON(resp.Body, &srverr); err != nil {
			return "", errors.Wrap(err, "problem parsing error message")
		}

		return "", srverr
	}

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "problem reading certificate from response")
	}

	return string(out), nil
}
