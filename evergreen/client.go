package evergreen

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

// Client holds the credentials for the Evergreen API.
type Client struct {
	apiRoot    string
	httpClient *http.Client
	user       string
	apiKey     string
}

// ConnectionInfo stores the root URL, username, and API key for the user
type ConnectionInfo struct {
	RootURL string `bson:"url" json:"url" yaml:"url"`
	User    string `bson:"user" json:"user" yaml:"user"`
	Key     string `bson:"key" json:"key" yaml:"key"`
}

// NewClient is a constructs a new Client using the parameters given.
func NewClient(httpClient *http.Client, info *ConnectionInfo) *Client {
	return &Client{
		apiRoot:    info.RootURL,
		httpClient: httpClient,
		user:       info.User,
		apiKey:     info.Key,
	}
}

// Checks that a user, API key, and root URL are given in the EvergreenInfo struct.
func (e *ConnectionInfo) IsValid() bool {
	if e == nil {
		return false
	}
	if e.RootURL == "" || e.User == "" || e.Key == "" {
		return false
	}
	return true
}

// getURL returns a URL for the given path.
func (c *Client) getURL(path string) string {
	return fmt.Sprintf("%s/api/rest/v2/%s", c.apiRoot, path)
}

// doReq performs a request of the given method type against path.
// If body is not nil, also includes it as a request body as url-encoded data
// with the appropriate header
func (c *Client) doReq(method, path string) (*http.Response, error) {
	var req *http.Request
	var err error

	startAt := time.Now()
	url := c.getURL(path)
	req, err = http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Api-Key", c.apiKey)
	req.Header.Add("Api-User", c.user)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		msg := fmt.Sprintf("empty response from server for %s request for URL %s", method, url)
		return nil, errors.New(msg)
	}
	if resp.StatusCode != http.StatusOK {
		msg := message.Fields{
			"status":   resp.Status,
			"code":     resp.StatusCode,
			"path":     url,
			"method":   method,
			"duration": time.Now().Sub(startAt).String(),
		}
		defer resp.Body.Close()
		if data, err := ioutil.ReadAll(resp.Body); err == nil {
			doc := struct {
				Error string
			}{}
			if err := json.Unmarshal(data, &doc); err == nil {
				msg["error"] = doc.Error
			}
		}

		grip.Warning(msg)
		return nil, errors.Errorf("http request failed with status %s", resp.Status)
	}

	return resp, nil
}

// getRel parses the result header Link to determine whether there is another
// page or this is the last page, and returns this keyword.
// Assumes that the url and rel keyword are separated by a semicolon, and that
// the rel keyword is encased in quotes.
func getRel(link string) (string, error) {
	links := strings.Split(link, ";")
	if len(links) < 2 {
		return "", errors.New("missing rel")
	}
	link = links[1]

	rels := strings.Split(link, "\"")
	if len(rels) < 2 {
		return "", errors.New("incorrect rel format")
	}
	rel := rels[1]
	if rel != "next" && rel != "prev" {
		return "", errors.New("error parsing link")
	}
	return rel, nil
}

// getPath parses the result header Link to find the next page's path.
// Assumes that the url is before a semicolon
func (c *Client) getPath(link string) (string, error) {
	link = strings.Split(link, ";")[0]
	start := 1
	end := len(link) - 1 //remove trailing >
	url := link[start:end]
	if !strings.HasPrefix(url, c.apiRoot) {
		return "", errors.New("Invalid link")
	}
	start = len(c.apiRoot)
	path := url[start:]
	return path, nil
}

// get performs a GET request for path, transforms the response body to JSON,
//and parses the link for the next page (this is empty if there is no next page)
func (c *Client) get(path string) ([]byte, string, error) {
	link := ""
	resp, err := c.doReq("GET", path)
	if err != nil {
		return nil, "", errors.WithStack(err)
	}
	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", errors.Wrap(err, "problem reading response")
	}

	links := resp.Header["Link"]
	if len(links) > 0 { //Paginated
		link = links[0]

		rel, err := getRel(link)
		if err != nil {
			return nil, "", errors.WithStack(err)
		}

		link, err = c.getPath(link)
		if err != nil {
			return nil, "", errors.WithStack(err)
		}

		// If the first link is "prev," we are at the end.
		if rel == "prev" {
			link = ""
		}
	}

	return out, link, nil
}
