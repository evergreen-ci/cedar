package evergreen

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

//Client holds the credentials for the Evergreen API
type Client struct {
	APIRoot    string
	httpClient *http.Client
	User       string
	APIKey     string
}

//Host holds information for a single host
type Host struct {
	HostID      string `json:"host_id"`
	Distro      Distro `json:"distro"`
	Provisioned bool   `json:"provisioned"`
	StartedBy   string `json:"started_by"`
	HostType    string `json:"host_type"`
	UserType    string `json:"user"`
	Status      string `json:"status"`
	RunningTask Task   `json:"running_task"`
}

//Distro holds information for a single distro within a host
type Distro struct {
	DistroID string `json:"distro_id"`
	Provider string `json:"provider"`
}

//Task holds information for a single task within a host
type Task struct {
	TaskID       string `json:"task_id"`
	Name         string `json:"name"`
	DispatchTime string `json:"dispatch_time"`
	VersionID    string `json:"version_id"`
	BuildID      string `json:"build_id"`
}

// getURL returns a URL for the given path.
func (c *Client) getURL(path string) string {
	return fmt.Sprintf("%s/%s", c.APIRoot, path)
}

// doReq performs a request of the given method type against path.
// If body is not nil, also includes it as a request body as url-encoded data
// with the appropriate header
func (c *Client) doReq(method, path string) (*http.Response, error) {
	var req *http.Request
	var err error
	url := c.getURL(path)
	req, err = http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Api-Key", c.APIKey)
	req.Header.Add("Api-User", c.User)
	resp, err := c.httpClient.Do(req)

	if err != nil {
		return nil, err
	}
	if resp == nil {
		msg := fmt.Sprintf("empty response from server for %s request for URL %s", method, url)
		return nil, errors.New(msg)
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
	if rel != "next" && rel != "last" {
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
	if !strings.HasPrefix(url, c.APIRoot) {
		return "", errors.New("Invalid link")
	}
	start = len(c.APIRoot)
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
		if rel == "last" {
			link = ""
		}
	}

	return out, link, nil
}

// GetHosts is an example wrapper function of get for a specific query.
// If limit is negative, GetHosts will return all hosts.
// Otherwise, will return the number of hosts <= limit.
func (c *Client) GetHosts(limit int) (<-chan *Host, <-chan error) {
	out := make(chan *Host)
	errs := make(chan error)

	// TODO: replace limit with contexts for cancelation
	go func() {
		seen := 0
		for {
			data, link, err := c.get("/hosts")
			if err != nil {
				errs <- err
				break
			}
			hosts := []*Host{}
			if err := json.Unmarshal(data, &hosts); err != nil {
				errs <- err
				break
			}

			for _, h := range hosts {
				seen++
				out <- h
			}

			if limit > 0 && seen >= limit {
				break
			}

			if link == "" {
				break
			}
		}
		close(out)
		close(errs)
	}()

	return out, errs
}
