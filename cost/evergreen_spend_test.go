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

// TestGetEvergreenData tests that GetEvergreenDistrosData runs without error
func (s *ClientSuite) TestGetEvergreenDataSuccess() {
	client := evergreen.NewClient(s.info.root, s.client, "", "")
	starttime, _ := time.Parse(time.RFC3339, "2017-07-01T00:00:00+00:00")
	duration, _ := time.ParseDuration("10h")
	distros, err := GetEvergreenDistrosData(client, starttime, duration)
	for _, d := range distros {
		s.NotEmpty(d)
	}
	s.NoError(err)
}
