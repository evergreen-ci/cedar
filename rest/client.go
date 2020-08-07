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

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	defaultClientPort int = 3000
	maxClientPort     int = 65535
)

// Client provides an interface for interacting with a remote amboy
// Service.
type Client struct {
	host     string
	prefix   string
	port     int
	username string
	apiKey   string
	client   *http.Client
}

type ClientOptions struct {
	Host     string
	Port     int
	Prefix   string
	Username string
	ApiKey   string
}

// NewClient takes host, port, and URI prefix information and
// constructs a new Client.
func NewClient(opts ClientOptions) (*Client, error) {
	c := &Client{client: &http.Client{}}

	return c.initClient(opts)
}

// NewClientFromExisting takes an existing http.Client object and
// produces a new Client object.
func NewClientFromExisting(client *http.Client, opts ClientOptions) (*Client, error) {
	if client == nil {
		return nil, errors.New("must use a non-nil existing client")
	}

	c := &Client{client: client}

	return c.initClient(opts)
}

// Copy takes an existing Client object and returns a new client
// object with the same settings that uses a *new* http.Client.
func (c *Client) Copy() *Client {
	new := &Client{}
	*new = *c
	new.client = &http.Client{}

	return new
}

func (c *Client) initClient(opts ClientOptions) (*Client, error) {
	if err := c.SetHost(opts.Host); err != nil {
		return nil, err
	}

	if opts.Port != 0 {
		if err := c.SetPort(opts.Port); err != nil {
			return nil, err
		}
	}

	if err := c.SetPrefix(opts.Prefix); err != nil {
		return nil, err
	}

	c.SetUser(opts.Username, opts.ApiKey)

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

func (c *Client) SetUser(u, k string) {
	c.username = u
	c.apiKey = k
}

func (c *Client) GetUser() string {
	return c.username
}

func (c *Client) GetApiKey() string {
	return c.apiKey
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

	if c.username != "" {
		req.Header[cedar.APIUserHeader] = []string{c.username}
	}
	if c.apiKey != "" {
		req.Header[cedar.APIKeyHeader] = []string{c.apiKey}
	}

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

	req, err := c.makeRequest(ctx, http.MethodGet, "/v1/admin/status", nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading status result")
	}

	return out, nil
}

///////////////////////////////////
//
// Simple Log Example Handler

func (c *Client) WriteSimpleLog(ctx context.Context, logID, data string, increment int) (*SimpleLogIngestionResponse, error) {
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

	out := &SimpleLogIngestionResponse{}

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

func (c *Client) GetRootCertificate(ctx context.Context) (string, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, c.getURL("/v1/admin/ca"), nil)
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
		if err = gimlet.GetJSON(resp.Body, &srverr); err != nil {
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

func (c *Client) authCredRequest(ctx context.Context, url, username, password, apiKey string) (string, error) {
	req, err := c.makeRequest(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", errors.Wrap(err, "problem building request")
	}

	if apiKey != "" {
		req.Header.Set(cedar.APIUserHeader, username)
		req.Header.Set(cedar.APIKeyHeader, apiKey)
	} else if password != "" {
		creds := userCredentials{Username: username, Password: password}
		var payload []byte
		payload, err = json.Marshal(creds)
		if err != nil {
			return "", errors.Wrap(err, "marshalling user credentials")
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(payload))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body []byte
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", errors.Wrap(err, "reading response body")
		}
		srverr := gimlet.ErrorResponse{}
		if err = json.Unmarshal(body, &srverr); err != nil {
			return "", errors.Errorf("received response: %s", body)
		}

		return "", srverr
	}

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "problem reading certificate from response")
	}

	return string(out), nil
}

func (c *Client) GetUserCertificate(ctx context.Context, username, password, apiKey string) (string, error) {
	return c.authCredRequest(ctx, c.getURL("/v1/admin/users/certificate"), username, password, apiKey)
}

func (c *Client) GetUserCertificateKey(ctx context.Context, username, password, apiKey string) (string, error) {
	return c.authCredRequest(ctx, c.getURL("/v1/admin/users/certificate/key"), username, password, apiKey)
}

func (c *Client) FindPerformanceResultById(ctx context.Context, id string) (*model.APIPerformanceResult, error) {
	url := c.getURL(fmt.Sprintf("/v1/perf/%s", id))

	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "problem building request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		srverr := gimlet.ErrorResponse{}
		if err = gimlet.GetJSON(resp.Body, &srverr); err != nil {
			return nil, errors.Wrap(err, "problem parsing error message")
		}

		return nil, srverr
	}

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}
	result := &model.APIPerformanceResult{}
	if err = json.Unmarshal(out, result); err != nil {
		return nil, errors.Wrap(err, "problem unmarshaling response data")
	}

	return result, nil
}

func (c *Client) RemovePerformanceResultById(ctx context.Context, id string) (string, error) {
	url := c.getURL(fmt.Sprintf("/v1/perf/%s", id))

	req, err := c.makeRequest(ctx, http.MethodDelete, url, nil)
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
		if err = gimlet.GetJSON(resp.Body, &srverr); err != nil {
			return "", errors.Wrap(err, "problem parsing error message")
		}

		return "", srverr
	}

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "problem reading response")
	}

	return string(out), nil
}
