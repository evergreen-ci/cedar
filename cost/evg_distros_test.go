package cost

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("cedar.evergreen.distros.test")
}

type DistrosSuite struct {
	client *http.Client
	info   *model.EvergreenConnectionInfo
	suite.Suite
}

func TestDistrosSuite(t *testing.T) {
	t.Skip("integration tests not supported by the service at this time")
	suite.Run(t, new(DistrosSuite))
}

func (s *DistrosSuite) SetupSuite() {
	s.info = &model.EvergreenConnectionInfo{
		RootURL: "https://evergreen.mongodb.com",
		User:    "USER",
		Key:     "KEY",
	}
	s.client = &http.Client{}
}

// TestGetDistrosFunction tests that GetDistros runs without error.
func (s *DistrosSuite) TestGetDistrosFunction() {
	Client := NewEvergreenClient(s.client, s.info)
	distros, err := Client.GetDistros(context.Background())
	s.True(len(distros) > 1)
	s.NoError(err)
}

// TestGetDistroFunctionFail tests against the local APIServer for correct
// behaviors given certain use cases.
func (s *DistrosSuite) TestGetDistroFunctionFail() {
	Client := NewEvergreenClient(s.client, s.info)
	Client.maxRetries = 2
	distroID := "archlinux-build"
	ctx := context.Background()
	t := time.Now().Add(-30 * 24 * time.Hour)

	// Test the case where the queried distro has tasks in the given time range.
	// This test will be implemented later when architectures for IntegrationTests
	// are in place for cedar.

	// Test the case where the queried distro has no tasks in the given time range,
	// by an invalid start time. GetDistro should succeed, but sumTimeTaken,
	// provider, intancetype must result in zero-values.
	dc, err := Client.GetDistroCost(ctx, distroID, t, 10*time.Minute)
	s.Require().NoError(err)
	s.Equal(distroID, dc.DistroID)

	// Test valid failures - i.e. searching for non-existent distros, invalid time.
	dc, err = Client.GetDistroCost(ctx, distroID, t, time.Hour)
	s.Error(err)
	s.Nil(dc)
	dc, err = Client.GetDistroCost(ctx, "fake", t, time.Hour)
	s.Error(err)
	s.Nil(dc)
	dc, err = Client.GetDistroCost(ctx, distroID, time.Time{}, 0)
	s.Error(err)
	s.Nil(dc)
}

// Tests GetEvergreenDistrosData(), which retrieves all distro costs
// for each distro in Evergreen. Authentication is needed for this route.
func (s *DistrosSuite) TestGetEvergreenDistrosData() {
	Client := NewEvergreenClient(s.client, s.info)
	Client.maxRetries = 2
	distroCosts, err := Client.GetEvergreenDistroCosts(context.Background(), time.Now().Add(-300*time.Hour), 48*time.Hour)
	s.NoError(err)
	for _, dc := range distroCosts {
		s.NotEmpty(dc.DistroID)
		s.NotEmpty(dc.SumTimeTakenMS)
	}
}
