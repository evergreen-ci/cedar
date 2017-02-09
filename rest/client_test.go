// +build go1.7

package rest

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	mgo "gopkg.in/mgo.v2"

	"github.com/mongodb/amboy/queue"
	"github.com/stretchr/testify/suite"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sink"
	"golang.org/x/net/context"
)

func init() {
	grip.SetThreshold(level.Debug)

}

type ClientSuite struct {
	service *Service
	client  *Client
	server  *httptest.Server
	info    struct {
		host string
		port int
	}
	closer context.CancelFunc
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupSuite() {
	ctx, cancel := context.WithCancel(context.Background())
	s.closer = cancel
	s.service = &Service{}
	require := s.Require()

	require.NoError(sink.SetQueue(queue.NewLocalUnordered(3)))
	require.NoError(s.service.Validate())
	require.NoError(s.service.Start(ctx))

	app := s.service.app
	s.NoError(app.Resolve())
	router, err := s.service.app.Router()
	s.NoError(err)
	s.server = httptest.NewServer(router)
	session, err := mgo.Dial("mongodb://localhost:27017")
	require.NoError(sink.SetMgoSession(session))

	portStart := strings.LastIndex(s.server.URL, ":")
	port, err := strconv.Atoi(s.server.URL[portStart+1:])
	require.NoError(err)
	s.info.host = s.server.URL[:portStart]
	s.info.port = port
	grip.Infof("running test rest service at '%s', on port '%d'", s.info.host, s.info.port)
}

func (s *ClientSuite) TearDownSuite() {
	grip.Infof("closing test rest service at '%s', on port '%d'", s.info.host, s.info.port)
	s.server.Close()
	s.closer()
}

func (s *ClientSuite) SetupTest() {
	s.client = &Client{}
}

////////////////////////////////////////////////////////////////////////
//
// A collection of tests that exercise and test the consistency and
// validation in the configuration interface for the rest client.
//
////////////////////////////////////////////////////////////////////////

func (s *ClientSuite) TestClientGetter() {
	s.Exactly(s.client.Client(), s.client.client)
}

func (s *ClientSuite) TestSetHostRequiresHttpURL() {
	example := "http://exmaple.com"

	s.Equal("", s.client.Host())
	s.NoError(s.client.SetHost(example))
	s.Equal(example, s.client.Host())

	badURI := []string{"foo", "1", "true", "htp", "ssh"}

	for _, uri := range badURI {
		s.Error(s.client.SetHost(uri))
		s.Equal(example, s.client.Host())
	}
}

func (s *ClientSuite) TestSetHostStripsTrailingSlash() {
	uris := []string{
		"http://foo.example.com/",
		"https://extra.example.net/bar/s/",
	}

	for _, uri := range uris {
		s.True(strings.HasSuffix(uri, "/"))
		s.NoError(s.client.SetHost(uri))
		s.Equal(uri[:len(uri)-1], s.client.Host())
		s.False(strings.HasSuffix(s.client.Host(), "/"))
	}
}

func (s *ClientSuite) TestSetHostRoundTripsValidHostWithGetter() {
	uris := []string{
		"http://foo.example.com",
		"https://extra.example.net/bar/s",
	}
	for _, uri := range uris {
		s.NoError(s.client.SetHost(uri))
		s.Equal(uri, s.client.Host())
	}
}

func (s *ClientSuite) TestPortSetterDisallowsPortsToBeZero() {
	s.Equal(0, s.client.port)
	s.Equal(0, s.client.Port())

	s.Error(s.client.SetPort(0))
	s.Equal(3000, s.client.Port())
}

func (s *ClientSuite) TestPortSetterDisallowsTooBigPorts() {
	s.Equal(0, s.client.port)
	s.Equal(0, s.client.Port())

	for _, p := range []int{65536, 70000, 1000000} {
		s.Error(s.client.SetPort(p), strconv.Itoa(p))
		s.Equal(3000, s.client.Port())
	}
}

func (s *ClientSuite) TestPortSetterRoundTripsValidPortsWithGetter() {
	for _, p := range []int{65, 8080, 1400} {
		s.NoError(s.client.SetPort(p), strconv.Itoa(p))
		s.Equal(p, s.client.Port())
	}
}

func (s *ClientSuite) TestSetPrefixRemovesTrailingAndLeadingSlashes() {
	s.Equal("", s.client.Prefix())

	for _, p := range []string{"/foo", "foo/", "/foo/"} {
		s.NoError(s.client.SetPrefix(p))
		s.Equal("foo", s.client.Prefix())
	}
}

func (s *ClientSuite) TestSetPrefixRoundTripsThroughGetter() {
	for _, p := range []string{"", "foo/bar", "foo", "foo/bar/baz"} {
		s.NoError(s.client.SetPrefix(p))
		s.Equal(p, s.client.Prefix())
	}
}

////////////////////////////////////////////////////////////////////////
//
// Client Initialization Checks/Tests
//
////////////////////////////////////////////////////////////////////////

func (s *ClientSuite) TestNewClientPropogatesValidValuesToCreatedValues() {
	nc, err := NewClient("http://example.com", 8080, "amboy")
	s.NoError(err)

	s.Equal(8080, nc.Port())
	s.Equal("http://example.com", nc.Host())
	s.Equal("amboy", nc.Prefix())
}

func (s *ClientSuite) TestCorrectedNewClientSettings() {
	nc, err := NewClient("http://example.com", 900000000, "/amboy/")
	s.Error(err)
	s.Nil(nc)
}

func (s *ClientSuite) TestNewClientConstructorPropogatesErrorStateForHost() {
	nc, err := NewClient("foo", 3000, "")

	s.Nil(nc)
	s.Error(err)
}

func (s *ClientSuite) TestNewClientFromExistingUsesExistinHTTPClient() {
	client := &http.Client{}

	nc, err := NewClientFromExisting(client, "http://example.com", 2048, "amboy")
	s.NoError(err)
	s.Exactly(client, nc.Client())
}

func (s *ClientSuite) TestNewClientFromExistingWithNilClientReturnsError() {
	nc, err := NewClientFromExisting(nil, "http://example.com", 2048, "amboy")
	s.Error(err)
	s.Nil(nc)
}

func (s *ClientSuite) TestCopyeConstructorUsesDifferentHTTPClient() {
	s.NotEqual(s.client.Client(), s.client.Copy().Client())
}

////////////////////////////////////////////////////////////////////////
//
// Client/Service Interaction: internals and helpers
//
////////////////////////////////////////////////////////////////////////

func (s *ClientSuite) TestURLGeneratiorWithoutDefaultPortInResult() {
	s.NoError(s.client.SetHost("http://amboy.example.net"))

	for _, p := range []int{0, 80} {
		s.client.port = p

		s.Equal("http://amboy.example.net/foo", s.client.getURL("foo"))
	}
}

func (s *ClientSuite) TestURLGenerationWithNonDefaultPort() {
	for _, p := range []int{82, 8080, 3000, 42420, 2048} {
		s.NoError(s.client.SetPort(p))
		host := "http://amboy.example.net"
		s.NoError(s.client.SetHost(host))
		prefix := "/queue"
		s.NoError(s.client.SetPrefix(prefix))
		endpoint := "/status"
		expected := strings.Join([]string{host, ":", strconv.Itoa(p), prefix, endpoint}, "")

		s.Equal(expected, s.client.getURL(endpoint))
	}
}

func (s *ClientSuite) TestURLGenerationWithEmptyPrefix() {
	host := "http://amboy.example.net"
	endpoint := "status"

	s.NoError(s.client.SetHost(host))
	s.Equal("", s.client.Prefix())

	s.Equal(strings.Join([]string{host, endpoint}, "/"),
		s.client.getURL(endpoint))
}
