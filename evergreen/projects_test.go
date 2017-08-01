package evergreen

import (
	"net/http"
	"testing"
	"time"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.evergreen.projects.test")
}

type ProjectSuite struct {
	client *http.Client
	info   struct {
		root string
		user string
		key  string
	}
	suite.Suite
}

func TestProjectsSuite(t *testing.T) {
	suite.Run(t, new(ProjectSuite))
}

func (s *ProjectSuite) SetupSuite() {
	// TODO: the following values are only for testing until production Evergreen
	// has the routes that we need.
	s.info.root = "http://localhost:8080/api/rest/v2/"
	s.info.user = "admin"
	s.info.key = "abb623665fdbf368a1db980dde6ee0f0"
	s.client = &http.Client{}
}

// Tests getProjects(), which retrieves all projects from Evergreen.
// Authentication is needed for this route.
func (s *ProjectSuite) TestGetProjects() {
	Client := NewClient(s.info.root, s.client, s.info.user, s.info.key)
	output := Client.getProjects()
	for out := range output {
		s.NoError(out.err)
		s.NotEmpty(out.output.Identifier)
	}
}

// Tests getTaskCostsByProject(), which retrieves all task costs from Evergreen
// for the project given. Authentication is needed for this route.
func (s *ProjectSuite) TestGetTaskCostsByProject() {
	Client := NewClient(s.info.root, s.client, s.info.user, s.info.key)
	// TODO: Right now the test is written for the locally running API server.
	// Change for the production evergreen later.
	output := Client.getTaskCostsByProject("amboy", "1970-01-01T00:00:00%2B00:00", "1h")
	for out := range output {
		s.NoError(out.err)
		s.NotEmpty(out.taskcost.TimeTaken)
	}
}

// Tests GetEvergreenProjectsData(), which retrieves all task costs from
// for each project in Evergreen. Authentication is needed for this route.
func (s *ProjectSuite) TestGetEvergreenProjectsData() {
	Client := NewClient(s.info.root, s.client, s.info.user, s.info.key)
	// TODO: Right now the test is written for the locally running API server.
	// Change for the production evergreen later.
	starttime, _ := time.Parse(time.RFC3339, "1970-01-01T00:00:00+00:00")
	duration, _ := time.ParseDuration("1h")
	projectUnits, err := Client.GetEvergreenProjectsData(starttime, duration)
	s.NoError(err)
	for _, pu := range projectUnits {
		s.NotEmpty(pu.Name)
		s.NotEmpty(pu.Tasks)
	}
}
