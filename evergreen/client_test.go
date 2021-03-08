package evergreen

import (
	"context"
	"net/http"
	"testing"

	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("cedar.evergreen.client.test")
}

type ClientSuite struct {
	client *http.Client
	info   *model.EvergreenConnectionInfo
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupSuite() {
	s.info = &model.EvergreenConnectionInfo{
		RootURL: "https://evergreen.mongodb.com",
	}
	s.client = &http.Client{}
}

func (s *ClientSuite) TestDoReqFunction() {
	client := NewClient(s.client, s.info)

	resp, err := client.doReq(context.TODO(), "GET", "/hosts")
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
	client := NewClient(s.client, s.info)
	link := "<https://evergreen.mongodb.com/rest/v2/hosts?limit=100>; rel=\"next\""
	path, err := client.getPath(link)
	s.Nil(err)
	s.Equal(path, "/rest/v2/hosts?limit=100")
}

func (s *ClientSuite) TestGetFunction() {
	client := NewClient(s.client, s.info)
	_, _, err := client.Get(context.Background(), "/hosts")
	s.Nil(err)
}
