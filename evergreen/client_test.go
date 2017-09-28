package evergreen

import (
	"net/http"
	"testing"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

func init() {
	grip.SetName("sink.evergreen.client.test")
}

type ClientSuite struct {
	client *http.Client
	info   *ConnectionInfo
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupSuite() {
	s.info = &ConnectionInfo{
		RootURL: "https://evergreen.mongodb.com",
	}
	s.client = &http.Client{}
}

func (s *ClientSuite) TestDoReqFunction() {
	//Construct Client

	Client := NewClient(s.client, s.info)

	resp, err := Client.doReq("GET", "/hosts")
	s.NoError(err)
	if s.NotNil(resp) {
		s.Equal(resp.StatusCode, 200)
	}
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
	s.Equal(path, "/rest/v2/hosts?limit=100")
	// TODO: ADD THIS BACK IN FOR PRODUCTION EVERGREEN.
	// link = "<https://thisiswrong.com/limit=100>; rel=\"next\""
	// _, err = Client.getPath(link)
	// s.NotNil(err)
}

func (s *ClientSuite) TestGetFunction() {
	Client := NewClient(s.client, s.info)
	_, _, err := Client.get(context.Background(), "/hosts")
	s.Nil(err)
}
