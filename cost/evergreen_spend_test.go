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
	info   struct {
		root string
		user string
		key  string
	}
	suite.Suite
}

func TestEvergreenSpendSuite(t *testing.T) {
	suite.Run(t, new(EvergreenSpendSuite))
}

func (s *EvergreenSpendSuite) SetupSuite() {
	// TODO: the following values are only for testing until production Evergreen
	// has the routes that we need.
	s.info.root = "http://localhost:8080/api/rest/v2/"
	s.info.user = "admin"
	s.info.key = "abb623665fdbf368a1db980dde6ee0f0"
	s.client = &http.Client{}
}

// // TestGetEvergreenDistrosData tests that GetEvergreenDistrosData
// // runs without error
// func (s *EvergreenSpendSuite) TestGetEvergreenDistrosData() {
// 	client := evergreen.NewClient(s.info.root, s.client, "", "")
// 	starttime, _ := time.Parse(time.RFC3339, "2017-07-01T00:00:00+00:00")
// 	duration, _ := time.ParseDuration("10h")
// 	distros, err := GetEvergreenDistrosData(client, starttime, duration)
// 	for _, d := range distros {
// 		s.NotEmpty(d)
// 	}
// 	s.NoError(err)
// }

func (s *EvergreenSpendSuite) TestGetEvergreenProjectsData() {
	client := evergreen.NewClient("http://localhost:8080/api/rest/v2/", s.client, "admin", "abb623665fdbf368a1db980dde6ee0f0")
	starttime, _ := time.Parse(time.RFC3339, "1970-01-01T00:00:00+00:00")
	duration, _ := time.ParseDuration("1h")
	projects, err := GetEvergreenProjectsData(client, starttime, duration)
	s.NoError(err)
	for _, p := range projects {
		s.NotEmpty(p.Name)
		for _, task := range p.Tasks {
			s.NotEmpty(task.Githash)
		}
	}
}

func (s *EvergreenSpendSuite) TestGetEvergreenData() {
	client := evergreen.NewClient("http://localhost:8080/api/rest/v2/", s.client, "admin", "abb623665fdbf368a1db980dde6ee0f0")
	starttime, _ := time.Parse(time.RFC3339, "1970-01-01T00:00:00+00:00")
	duration, _ := time.ParseDuration("1h")
	evergreen, err := GetEvergreenData(client, starttime, duration)
	s.NoError(err)
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
