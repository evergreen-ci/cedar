package sink

import (
	"testing"

	"github.com/mongodb/amboy/queue"
	"github.com/stretchr/testify/suite"
)

type ServiceCacheSuite struct {
	cache *appServicesCache
	suite.Suite
}

func TestServiceCacheSuite(t *testing.T) {
	suite.Run(t, new(ServiceCacheSuite))
}

func (s *ServiceCacheSuite) SetupTest() {
	s.cache = &appServicesCache{name: "sink.testing"}
}

func (s *ServiceCacheSuite) TestDefaultCacheValues() {
	s.Nil(s.cache.queue)
	s.Equal("sink.testing", s.cache.name)
	s.Nil(s.cache.session)
}

func (s *ServiceCacheSuite) TestQueueNotSettableToNil() {
	s.Error(s.cache.setQueue(nil))
	s.Nil(s.cache.queue)

	q := queue.NewLocalOrdered(2)
	s.NotNil(q)
	s.NoError(s.cache.setQueue(q))
	s.NotNil(s.cache.queue)
	s.Equal(s.cache.queue, q)
	s.Error(s.cache.setQueue(nil))
	s.NotNil(s.cache.queue)
	s.Equal(s.cache.queue, q)
}

func (s *ServiceCacheSuite) TestQueueGetterRetrivesQueue() {
	q, err := s.cache.getQueue()
	s.Nil(q)
	s.Error(err)

	q = queue.NewLocalOrdered(2)
	s.NoError(s.cache.setQueue(q))

	retrieved, err := s.cache.getQueue()
	s.NotNil(q)
	s.NoError(err)
	s.Equal(retrieved, q)
}

func (s *ServiceCacheSuite) TestSetSpendConfig() {
	file := "cost/testdata/spend_test.yml"
	err := s.cache.setSpendConfig(file)
	s.NoError(err)
	configFile := s.cache.spendConfig
	s.Equal(configFile.Opts.Duration, "8h")

	file = "not_real.yaml"
	err = s.cache.setSpendConfig(file)
	s.Error(err)
}
