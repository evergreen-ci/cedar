package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/jpillora/backoff"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

// EvergreenClient holds the credentials for the Evergreen API.
type EvergreenClient struct {
	apiRoot                string
	httpClient             *http.Client
	user                   string
	apiKey                 string
	allowIncompleteResults bool
	maxRetries             int
}

// NewEvergreenClient is a constructs a new Client using the parameters given.
func NewEvergreenClient(httpClient *http.Client, info *model.EvergreenConnectionInfo) *EvergreenClient {
	return &EvergreenClient{
		apiRoot:    info.RootURL,
		httpClient: httpClient,
		user:       info.User,
		apiKey:     info.Key,
		maxRetries: 5,
	}
}

func (c *EvergreenClient) SetAllowIncompleteResults(shouldAllow bool) {
	c.allowIncompleteResults = shouldAllow
}

// getURL returns a URL for the given path.
func (c *EvergreenClient) getURL(path string) string {
	if strings.HasPrefix(path, "/rest/v2/") {
		return c.apiRoot + path
	}

	return strings.Join([]string{c.apiRoot, "rest", "v2", strings.TrimPrefix(path, "/")}, "/")
}

func (c *EvergreenClient) getBackoff() *backoff.Backoff {
	return &backoff.Backoff{
		Min:    250 * time.Millisecond,
		Max:    5 * time.Second,
		Factor: 2,
		Jitter: true,
	}
}

func (c *EvergreenClient) retryRequest(ctx context.Context, method, path string) (*http.Response, error) {
	if method == "" || path == "" {
		return nil, errors.New("invalid request")
	}

	backoff := c.getBackoff()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for i := 0; i < c.maxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, errors.New("request canceled")
		case <-timer.C:
			resp, err := c.doReq(ctx, method, path)
			if err != nil {
				grip.Warningf("request %s of %s encountered error '%v'; retrying", method, path, err)
			} else if resp == nil {
				grip.Warningf("request %s of %s encountered nil result; retrying", method, path)
			} else if resp.StatusCode == http.StatusOK {
				return resp, nil
			} else if resp.StatusCode == http.StatusNotFound {
				return nil, errors.Errorf("%s resource (%s) is not found", path, method)
			} else if resp.StatusCode == http.StatusUnauthorized {
				return nil, errors.Errorf("access denied for %s of %s", method, path)
			} else { // nolint
				grip.Infof("problem with status %s on request %s of %s, retrying", resp.StatusCode, method, path)
			}

			timer.Reset(backoff.Duration())
		}
	}

	return nil, errors.Errorf("%d of %d reached maximum retries", c.maxRetries, c.maxRetries)
}

// doReq performs a request of the given method type against path.
// If body is not nil, also includes it as a request body as url-encoded data
// with the appropriate header
func (c *EvergreenClient) doReq(ctx context.Context, method, path string) (*http.Response, error) {
	var req *http.Request
	var err error

	startAt := time.Now()
	url := c.getURL(path)
	grip.Info(message.Fields{
		"method": method,
		"url":    url,
		"user":   c.user,
	})

	req, err = http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Api-Key", c.apiKey)
	req.Header.Add("Api-User", c.user)
	req = req.WithContext(ctx)
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
			"user":     c.user,
			"duration": time.Since(startAt).String(),
		}
		defer resp.Body.Close()
		if data, err := ioutil.ReadAll(resp.Body); err == nil {
			doc := struct {
				Error string
			}{}
			if err := json.Unmarshal(data, &doc); err == nil {
				msg["error"] = doc.Error
			} else {
				msg["body"] = string(data)
			}
		}

		grip.Warning(msg)
		return nil, errors.Errorf("http request failed with status %s", resp.Status)
	}

	grip.Info(message.Fields{
		"method":   method,
		"url":      url,
		"user":     c.user,
		"duration": time.Since(startAt).String(),
	})

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
func (c *EvergreenClient) getPath(link string) (string, error) {
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
func (c *EvergreenClient) get(ctx context.Context, path string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	link := ""
	path = strings.TrimRight(path, ":")
	resp, err := c.retryRequest(ctx, "GET", path)
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
