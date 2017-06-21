package evergreen

import (
	"net/http"
	"testing"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.cost.test")
}

type ClientSuite struct {
	client *http.Client
	info   struct {
		root string
		user string
		key  string
	}
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupSuite() {
	s.info.root = "https://evergreen.mongodb.com/rest/v2/"
	s.client = &http.Client{}
}

func (s *ClientSuite) TestDoReqFunction() {
	//Construct Client
	Client := &Client{
		APIRoot:    s.info.root,
		httpClient: s.client,
	}

	resp, err := Client.doReq("GET", "/hosts")
	s.Nil(err)
	s.Equal(resp.StatusCode, 200)

}
func (s *ClientSuite) TestGetRelFunction() {
	link := "https://Thisisatest.com; rel=\"next\""
	rel, err := getRel(link)
	s.Nil(err)
	s.Equal(rel, "next")
	link = "'https://Thisisatest.com; rel=\"last\""
	rel, err = getRel(link)
	s.Nil(err)
	s.Equal(rel, "last")
	link = "'https://Thisisatest.com; rel=\"false\""
	_, err = getRel(link)
	s.NotNil(err)
}

func (s *ClientSuite) TestGetPathFunction() {
	Client := &Client{
		APIRoot:    s.info.root,
		httpClient: s.client,
	}
	link := "<https://evergreen.mongodb.com/rest/v2/hosts?limit=100>; rel=\"next\""
	path, err := Client.getPath(link)
	s.Nil(err)
	s.Equal(path, "hosts?limit=100")
	link = "<https://thisiswrong.com/limit=100>; rel=\"next\""
	_, err = Client.getPath(link)
	s.NotNil(err)
}

func (s *ClientSuite) TestGetFunction() {
	Client := &Client{
		APIRoot:    s.info.root,
		httpClient: s.client,
	}
	_, _, err := Client.get("/hosts")
	s.Nil(err)
}

func (s *ClientSuite) TestGetHostsFunction() {
	Client := &Client{
		APIRoot:    s.info.root,
		httpClient: s.client,
	}
	hosts, errs := Client.GetHosts(300)
	count := 0
	for h := range hosts {
		count++
		s.NotEmpty(h.HostID)
	}

	for err := range errs {
		s.NoError(err)
	}

	s.Equal(count, 300)
}
