package evergreen

import (
	"net/http"
	"testing"
	"time"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.evergreen.distros.test")
}

type DistrosSuite struct {
	client *http.Client
	info   struct {
		root string
		user string
		key  string
	}
	suite.Suite
}

func TestDistrosSuite(t *testing.T) {
	suite.Run(t, new(DistrosSuite))
}

func (s *DistrosSuite) SetupSuite() {
	s.info.root = "https://evergreen-staging.corp.mongodb.com/rest/v2/"
	s.info.user = "USER"
	s.info.key = "KEY"
	s.client = &http.Client{}
}

// TestGetDistrosFunction tests that GetDistros runs without error.
func (s *DistrosSuite) TestGetDistrosFunction() {
	Client := NewClient(s.info.root, s.client, s.info.user, s.info.key)
	distros, err := Client.GetDistros()
	for _, d := range distros {
		s.NotEmpty(d.DistroID)
	}
	s.NoError(err)
}

// TestGetDistroFunctionFail tests against the local APIServer for correct
// behaviors given certain use cases.
func (s *DistrosSuite) TestGetDistroFunctionFail() {
	Client := NewClient(s.info.root, s.client, s.info.user, s.info.key)
	distroID := "archlinux-build"

	// Test the case where the queried distro has tasks in the given time range.
	// This test will be implemented later when architectures for IntegrationTests
	// are in place for sink.

	// Test the case where the queried distro has no tasks in the given time range,
	// by an invalid start time. GetDistro should succeed, but sumTimeTaken,
	// provider, intancetype must result in zero-values.
	dc, err := Client.GetDistroCost(distroID, "2017-07-27T10:00:00Z", "48h")
	s.NoError(err)
	s.Equal(distroID, dc.DistroID)

	// Test valid failures - i.e. searching for non-existent distros, invalid time.
	dc, err = Client.GetDistroCost(distroID, "2017-07-19T19:37:53", "1h")
	s.Error(err)
	dc, err = Client.GetDistroCost("fake", "2017-07-19T19:37:53Z", "1h")
	s.Error(err)
	dc, err = Client.GetDistroCost(distroID, "", "")
	s.Error(err)
}

// Tests GetEvergreenDistrosData(), which retrieves all distro costs
// for each distro in Evergreen. Authentication is needed for this route.
func (s *DistrosSuite) TestGetEvergreenDistrosData() {
	Client := NewClient(s.info.root, s.client, s.info.user, s.info.key)
	starttime, _ := time.Parse(time.RFC3339, "2017-07-31T10:00:00Z")
	duration, _ := time.ParseDuration("48h")
	distroCosts, err := Client.GetEvergreenDistrosData(starttime, duration)
	s.NoError(err)
	for _, dc := range distroCosts {
		s.NotEmpty(dc.DistroID)
		s.NotEmpty(dc.SumTimeTaken)
	}
}
