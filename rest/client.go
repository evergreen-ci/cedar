package rest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/gimlet"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

const (
	defaultClientPort int = 3000
	maxClientPort         = 65535
	jsonMimeType          = "application/json"
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
	var err error

	err = c.SetHost(host)
	if err != nil {
		return nil, err
	}

	err = c.SetPort(port)
	if err != nil {
		return nil, err
	}

	err = c.SetPrefix(prefix)
	if err != nil {
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

////////////////////////////////////////////////////////////////////////
//
// Public Operations that Interact with the Service
//
////////////////////////////////////////////////////////////////////////

func (c *Client) GetStatus(ctx context.Context) (*StatusResponse, error) {
	out := &StatusResponse{}
	url := c.getURL("/v1/status")
	grip.Debugln("GET", url)
	resp, err := ctxhttp.Get(ctx, c.client, url)
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
	url := c.getURL(fmt.Sprintf("/v1/simple_log/%s", logID))

	req := &simpleLogRequest{
		Time:      time.Now(),
		Increment: increment,
		Content:   data,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "problem converting json")
	}

	grip.Debugln("POST", url)
	resp, err := ctxhttp.Post(ctx, c.client, url, jsonMimeType, bytes.NewBuffer(payload))
	grip.Debugf("%+v", resp)
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
	url := c.getURL(fmt.Sprintf("/v1/simple_log/%s", logID))
	out := &SimpleLogContentResponse{}

	grip.Debugln("GET", url)
	resp, err := ctxhttp.Get(ctx, c.client, url)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}
	defer resp.Body.Close()

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading simple log result")
	}

	return out, nil
}

func (c *Client) GetSimpleLogText(ctx context.Context, logID string) ([]string, error) {
	url := c.getURL(fmt.Sprintf("/v1/simple_log/%s/text", logID))

	grip.Debugln("GET", url)
	resp, err := ctxhttp.Get(ctx, c.client, url)
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
	url := c.getURL(fmt.Sprintf("/v1/status/events/%s?limit=%d", level, limit))
	out := &SystemEventsResponse{}

	grip.Debugln("GET", url)
	resp, err := ctxhttp.Get(ctx, c.client, url)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}

	defer resp.Body.Close()

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system status result")
	}

	return out, nil
}

func (c *Client) GetSystemEvent(ctx context.Context, id string) (*SystemEventResponse, error) {
	url := c.getURL("/v1/status/events/" + id)
	out := &SystemEventResponse{}
	grip.Debugln("GET", url)

	resp, err := ctxhttp.Get(ctx, c.client, url)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}

	defer resp.Body.Close()

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.Wrap(err, "problem reading system status result")
	}

	return out, nil
}

func (c *Client) AcknowledgeSystemEvent(ctx context.Context, id string) (*SystemEventResponse, error) {
	url := c.getURL(fmt.Sprintf("/v1/status/events/%s/acknowledge", id))
	out := &SystemEventResponse{}
	grip.Debugln("POST", url)

	resp, err := ctxhttp.Post(ctx, c.client, url, "", nil)
	if err != nil {
		return nil, errors.Wrap(err, "problem with request")
	}

	defer resp.Body.Close()

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

	url := c.getURL("/v1/system_info")
	grip.Debugln("POST", url)
	resp, err := ctxhttp.Post(ctx, c.client, url, jsonMimeType, bytes.NewBuffer(payload))
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

	grip.Debugln("GET", url)
	resp, err := ctxhttp.Get(ctx, c.client, url)
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
