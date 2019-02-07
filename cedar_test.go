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
	s.Nil(s.cache.localQueue)
	s.Nil(s.cache.remoteQueue)
	s.Nil(s.cache.remoteReporter)
	s.Equal("cedar.testing", s.cache.name)
	s.Nil(s.cache.session)
}

func (s *ServiceCacheSuite) TestLocalQueueNotSettableToNil() {
	s.Error(s.cache.SetLocalQueue(nil))
	s.Nil(s.cache.localQueue)

	q := queue.NewLocalOrdered(2)
	s.NotNil(q)
	s.NoError(s.cache.SetLocalQueue(q))
	s.NotNil(s.cache.localQueue)
	s.Equal(s.cache.localQueue, q)
	s.Error(s.cache.SetLocalQueue(nil))
	s.NotNil(s.cache.localQueue)
	s.Equal(s.cache.localQueue, q)
}

func (s *ServiceCacheSuite) TestRemoteQueueNotSettableToNil() {
	s.Error(s.cache.SetRemoteQueue(nil))
	s.Nil(s.cache.remoteQueue)

	q := queue.NewLocalUnordered(2) // for testing
	s.NotNil(q)
	s.NoError(s.cache.SetRemoteQueue(q))
	s.NotNil(s.cache.remoteQueue)
	s.Equal(s.cache.remoteQueue, q)
	s.Error(s.cache.SetRemoteQueue(nil))
	s.NotNil(s.cache.remoteQueue)
	s.Equal(s.cache.remoteQueue, q)

	s.Nil(s.cache.remoteReporter)
}

func (s *ServiceCacheSuite) TestQueueGetterRetrievesQueue() {
	q, err := s.cache.GetLocalQueue()
	s.Nil(q)
	s.Error(err)

	q, err = s.cache.GetRemoteQueue()
	s.Nil(q)
	s.Error(err)

	q = queue.NewLocalOrdered(2)
	s.NoError(s.cache.SetLocalQueue(q))

	retrieved, err := s.cache.GetLocalQueue()
	s.NotNil(q)
	s.NoError(err)
	s.Equal(retrieved, q)
}
