package cedar

import (
	"testing"

	"github.com/mongodb/amboy/queue"
	"github.com/stretchr/testify/suite"
)

type ServiceCacheSuite struct {
	cache *envState
	suite.Suite
}

func TestServiceCacheSuite(t *testing.T) {
	suite.Run(t, new(ServiceCacheSuite))
}

func (s *ServiceCacheSuite) SetupTest() {
	s.cache = &envState{name: "cedar.testing"}
}

func (s *ServiceCacheSuite) TestDefaultCacheValues() {
	s.Nil(s.cache.queue)
	s.Equal("cedar.testing", s.cache.name)
	s.Nil(s.cache.session)
}

func (s *ServiceCacheSuite) TestQueueNotSettableToNil() {
	s.Error(s.cache.SetQueue(nil))
	s.Nil(s.cache.queue)

	q := queue.NewLocalOrdered(2)
	s.NotNil(q)
	s.NoError(s.cache.SetQueue(q))
	s.NotNil(s.cache.queue)
	s.Equal(s.cache.queue, q)
	s.Error(s.cache.SetQueue(nil))
	s.NotNil(s.cache.queue)
	s.Equal(s.cache.queue, q)
}

func (s *ServiceCacheSuite) TestQueueGetterRetrivesQueue() {
	q, err := s.cache.GetQueue()
	s.Nil(q)
	s.Error(err)

	q = queue.NewLocalOrdered(2)
	s.NoError(s.cache.SetQueue(q))

	retrieved, err := s.cache.GetQueue()
	s.NotNil(q)
	s.NoError(err)
	s.Equal(retrieved, q)
}
