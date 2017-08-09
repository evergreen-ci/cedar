package evergreen

import (
	"net/http"
	"testing"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.evergreen.client.test")
}

type ClientSuite struct {
	client *http.Client
	info   *EvergreenInfo
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupSuite() {
	s.info = &EvergreenInfo{
		RootURL: "https://evergreen.mongodb.com/rest/v2/",
	}
	s.client = &http.Client{}
}

func (s *ClientSuite) TestDoReqFunction() {
	//Construct Client

	Client := NewClient(s.client, s.info)

	resp, err := Client.doReq("GET", "/hosts")
	s.Nil(err)
	s.Equal(resp.StatusCode, 200)
}

func (s *ClientSuite) TestGetRelFunction() {
	link := "https://Thisisatest.com; rel=\"next\""
	rel, err := getRel(link)
	s.Nil(err)
	s.Equal(rel, "next")
	link = "'https://Thisisatest.com; rel=\"prev\""
	rel, err = getRel(link)
	s.Nil(err)
	s.Equal(rel, "prev")
	link = "'https://Thisisatest.com; rel=\"false\""
	_, err = getRel(link)
	s.NotNil(err)
}

func (s *ClientSuite) TestGetPathFunction() {
	Client := NewClient(s.client, s.info)
	link := "<https://evergreen.mongodb.com/rest/v2/hosts?limit=100>; rel=\"next\""
	path, err := Client.getPath(link)
	s.Nil(err)
	s.Equal(path, "hosts?limit=100")
	// TODO: ADD THIS BACK IN FOR PRODUCTION EVERGREEN.
	// link = "<https://thisiswrong.com/limit=100>; rel=\"next\""
	// _, err = Client.getPath(link)
	// s.NotNil(err)
}

func (s *ClientSuite) TestGetFunction() {
	Client := NewClient(s.client, s.info)
	_, _, err := Client.get("/hosts")
	s.Nil(err)
}
