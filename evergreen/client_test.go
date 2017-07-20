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
	Client := NewClient(s.info.root, s.client, "", "")

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
	Client := NewClient(s.info.root, s.client, "", "")
	link := "<https://evergreen.mongodb.com/rest/v2/hosts?limit=100>; rel=\"next\""
	path, err := Client.getPath(link)
	s.Nil(err)
	s.Equal(path, "hosts?limit=100")
	link = "<https://thisiswrong.com/limit=100>; rel=\"next\""
	_, err = Client.getPath(link)
	s.NotNil(err)
}

func (s *ClientSuite) TestGetFunction() {
	Client := NewClient(s.info.root, s.client, "", "")
	_, _, err := Client.get("/hosts")
	s.Nil(err)
}

// TestGetDistrosFunction tests that GetDistros runs without error.
func (s *ClientSuite) TestGetDistrosFunction() {
	Client := NewClient(s.info.root, s.client, "", "")
	distros, err := Client.GetDistros()
	for _, d := range distros {
		s.NotEmpty(d.DistroID)
	}
	s.NoError(err)
}

// TestGetDistroFunctionFail tests against the local APIServer for correct
// behaviors given certain use cases.
func (s *ClientSuite) TestGetDistroFunctionFail() {
	Client := NewClient(s.info.root, s.client, "", "")
	distroID := "archlinux-build"

	// Test the case where the queried distro has tasks in the given time range.
	// This test will be implemented later when architectures for IntegrationTests
	// are in place for sink.

	// Test the case where the queried distro has no tasks in the given time range,
	// by an invalid start time. GetDistro should succeed, but sumTimeTaken,
	// provider, intancetype must result in zero-values.
	dc, err := Client.GetDistroCost(distroID, "2017-07-19T19:37:53%2B00:00", "1h")
	s.NoError(err)
	s.Equal(distroID, dc.DistroID)

	// Test valid failures - i.e. searching for non-existent distros, invalid time.
	dc, err = Client.GetDistroCost(distroID, "2017-07-19T19:37:53", "1h")
	s.Error(err)
	dc, err = Client.GetDistroCost("fake", "2017-07-19T19:37:53%2B00:00", "1h")
	s.Error(err)
	dc, err = Client.GetDistroCost(distroID, "", "")
	s.Error(err)
}
