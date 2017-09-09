package cost

import (
	"net/http"
	"testing"
	"time"

	"github.com/evergreen-ci/sink/evergreen"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.evergreen.cost.test")
}

type EvergreenSpendSuite struct {
	client *http.Client
	info   *evergreen.ConnectionInfo
	suite.Suite
}

func TestEvergreenSpendSuite(t *testing.T) {
	t.Skip("integration tests not supported by the service at this time")
	suite.Run(t, new(EvergreenSpendSuite))
}

func (s *EvergreenSpendSuite) SetupSuite() {
	s.info = &evergreen.ConnectionInfo{
		RootURL: "https://evergreen.mongodb.com/rest/v2/",
		User:    "USER",
		Key:     "KEY",
	}
	s.client = &http.Client{}
}

// TestGetEvergreenDistrosData tests that GetEvergreenDistrosData
// runs without error
func (s *EvergreenSpendSuite) TestGetEvergreenDistrosData() {
	client := evergreen.NewClient(s.client, s.info)
	starttime, _ := time.Parse(time.RFC3339, "2017-07-25T10:00:00Z")
	duration, _ := time.ParseDuration("48h")
	distros, err := getEvergreenDistrosData(client, starttime, duration)

	for _, d := range distros {
		s.NotEmpty(d)
	}
	s.NoError(err)
}

// TestGetEvergreenDistrosData tests that GetEvergreenProjectsData
// runs without error
func (s *EvergreenSpendSuite) TestGetEvergreenProjectsData() {
	client := evergreen.NewClient(s.client, s.info)
	starttime, _ := time.Parse(time.RFC3339, "2017-07-25T10:00:00Z")
	duration, _ := time.ParseDuration("1h")
	projects, err := getEvergreenProjectsData(client, starttime, duration)
	s.NoError(err)
	for _, p := range projects {
		s.NotEmpty(p.Name)
		for _, task := range p.Tasks {
			s.NotEmpty(task.Githash)
		}
	}
}

func (s *EvergreenSpendSuite) TestGetEvergreenData() {
	client := evergreen.NewClient(s.client, s.info)
	starttime, _ := time.Parse(time.RFC3339, "2017-07-25T10:00:00Z")
	duration, _ := time.ParseDuration("4h")
	evergreen, err := getEvergreenData(client, starttime, duration)
	s.NotNil(evergreen)
	if !s.NoError(err) {
		return
	}
	for _, p := range evergreen.Projects {
		s.NotEmpty(p.Name)
		for _, task := range p.Tasks {
			s.NotEmpty(task.Githash)
		}
	}
	for _, d := range evergreen.Distros {
		s.NotEmpty(d.Name)
	}
}
