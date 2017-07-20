package evergreen

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

//Client holds the credentials for the Evergreen API
type Client struct {
	apiRoot    string
	httpClient *http.Client
	user       string
	apiKey     string
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
	DistroID string `json:"_id"`
}

type DistroCost struct {
	DistroID     string        `json:"distro_id"`
	Provider     string        `json:"provider"`
	InstanceType string        `json:"instance_type,omitempty"`
	SumTimeTaken time.Duration `json:"sum_time_taken"`
}

//Task holds information for a single task within a host
type Task struct {
	Githash      string        `json:"githash"`
	DisplayName  string        `json:"display_name"`
	DistroID     string        `json:"distro"`
	BuildVariant string        `json:"build_variant"`
	TimeTaken    time.Duration `json:"time_taken"`
}

func NewClient(apiRoot string, httpClient *http.Client, user string, apiKey string) *Client {
	return &Client{
		apiRoot:    apiRoot,
		httpClient: httpClient,
		user:       user,
		apiKey:     apiKey,
	}
}

// getURL returns a URL for the given path.
func (c *Client) getURL(path string) string {
	return fmt.Sprintf("%s/%s", c.apiRoot, path)
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
		return nil, errors.New("http request failed (not 200 OK)")
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
		if rel == "last" {
			link = ""
		}
	}

	return out, link, nil
}

// GetDistros is a wrapper function of get for a getting all distros from the
// Evergreen API.
func (c *Client) GetDistros() ([]*Distro, error) {
	data, link, err := c.get("/distros")
	if link != "" {
		return nil, errors.New("/distros should not be a paginated route")
	}
	if err != nil {
		return nil, errors.Wrap(err, "error in getting distros")
	}
	distros := []*Distro{}
	if err := json.Unmarshal(data, &distros); err != nil {
		return nil, err
	}
	return distros, nil
}

// GetDistroCost is a wrapper function of get for a getting all distro costs
// from the evergreen API.
func (c *Client) GetDistroCost(distroId, starttime, duration string) (*DistroCost, error) {
	data, link, err := c.get("/cost/distro/" + distroId +
		"?starttime=" + starttime + "&duration=" + duration)
	if link != "" {
		return nil, errors.New("/cost/distro should not be a paginated route")
	}
	if err != nil {
		return nil, errors.Wrap(err, "error in GetDistroCost")
	}
	distro := &DistroCost{}
	if err := json.Unmarshal(data, &distro); err != nil {
		return nil, err
	}
	return distro, nil
}

// A helper function for GetEvergreenDistrosData that gets distroID of
// all distros by calling GetDistros.
func (c *Client) getDistroIDs() ([]string, error) {
	distroIDs := []string{}
	distros, err := c.GetDistros()
	if err != nil {
		return nil, errors.Wrap(err, "error getting distros ids")
	}
	for _, d := range distros {
		distroIDs = append(distroIDs, d.DistroID)
	}
	return distroIDs, nil
}

// A helper function for GetEvergreenDistrosData that gets provider,
// instance type, and total time for a given list of distros found.
func (c *Client) getDistroCosts(distroIDs []string, st, dur string) ([]*DistroCost, error) {
	distroCosts := []*DistroCost{}
	for _, distro := range distroIDs {
		evgdc, err := c.GetDistroCost(distro, st, dur)
		if err != nil {
			return nil, errors.Wrap(err, "error when getting distro cost data from Evergreen")
		}
		distroCosts = append(distroCosts, evgdc)
	}
	return distroCosts, nil
}

// GetEvergreenDistrosData retrieves distros cost data from Evergreen.
func (c *Client) GetEvergreenDistrosData(starttime time.Time, duration time.Duration) ([]*DistroCost, error) {
	st := starttime.Format(time.RFC3339)
	dur := duration.String()

	distroIDs, err := c.getDistroIDs()
	if err != nil {
		return nil, errors.Wrap(err, "error in getting distroID in GetEvergreenDistrosData")
	}

	distroCosts, err := c.getDistroCosts(distroIDs, st, dur)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting distro costs in GetEvergreenDistrosData")
	}

	return distroCosts, nil
}
