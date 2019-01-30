package certdepot

import (
	"testing"
	"time"

	"github.com/square/certstrap/depot"
	"github.com/stretchr/testify/suite"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type MongoDepotTestSuite struct {
	session        *mgo.Session
	collection     *mgo.Collection
	databaseName   string
	collectionName string
	mongoDepot     *mongoCertDepot

	suite.Suite
}

func TestMongoDepot(t *testing.T) {
	s := &MongoDepotTestSuite{}
	s.setup()
	suite.Run(t, s)
}

func (s *MongoDepotTestSuite) TearDownTest() {
	err := s.collection.DropCollection()
	if err != nil {
		s.Require().Equal("ns not found", err.Error())
	}
}

func (s *MongoDepotTestSuite) setup() {
	var err error
	s.session, err = mgo.DialWithTimeout("mongodb://localhost:27017", 2*time.Second)
	s.Require().NoError(err)
	s.session.SetSocketTimeout(time.Hour)
	s.databaseName = "certDepot"
	s.collectionName = "certs"
	s.collection = s.session.DB(s.databaseName).C(s.collectionName)
	s.TearDownTest()
	s.mongoDepot = &mongoCertDepot{
		session:        s.session,
		databaseName:   s.databaseName,
		collectionName: s.collectionName,
		expireAfter:    30 * 24 * time.Hour,
	}
}

func (s *MongoDepotTestSuite) TestPut() {
	name := "bob"
	data := []byte("bob's fake certificate")

	// put fails with nil data
	s.Error(s.mongoDepot.Put(depot.CrtTag(name), nil))

	// put correctly adds data
	beforeTime := time.Now()
	time.Sleep(time.Second)
	s.NoError(s.mongoDepot.Put(depot.CrtTag(name), data))
	u := &DBUser{}
	s.Require().NoError(s.collection.FindId(name).One(u))
	s.Equal(name, u.ID)
	s.Equal(string(data), u.Cert)
	s.True(beforeTime.Before(u.TTL))

	// put correctly updates data
	beforeTime = time.Now()
	time.Sleep(time.Second)
	newData := []byte("bob's new fake certificate")
	s.NoError(s.mongoDepot.Put(depot.CrtTag(name), newData))
	u = &DBUser{}
	s.Require().NoError(s.collection.FindId(name).One(u))
	s.Equal(name, u.ID)
	s.Equal(string(newData), u.Cert)
	s.True(beforeTime.Before(u.TTL))
}

func (s *MongoDepotTestSuite) TestCheck() {
	name := "alice"
	data := []byte("alice's fake certificate")
	u := &DBUser{
		ID:   name,
		Cert: string(data),
		TTL:  time.Now(),
	}

	// check returns false when a user does not exist
	s.False(s.mongoDepot.Check(depot.CrtTag(name)))

	// check returns true when a user exists
	s.Require().NoError(s.collection.Insert(u))
	s.True(s.mongoDepot.Check(depot.CrtTag(name)))
}

func (s *MongoDepotTestSuite) TestGet() {
	name := "bob"
	expectedData := []byte("bob's fake certificate")
	u := &DBUser{
		ID:   name,
		Cert: string(expectedData),
		TTL:  time.Now(),
	}

	// get returns an error when the user does not exist
	actualData, err := s.mongoDepot.Get(depot.CrtTag(name))
	s.Error(err)
	s.Nil(actualData)

	// get returns correct certificate when user exists
	s.Require().NoError(s.collection.Insert(u))
	actualData, err = s.mongoDepot.Get(depot.CrtTag(name))
	s.NoError(err)
	s.Equal(expectedData, actualData)

	// get errors with expired TTL
	_, err = s.collection.UpsertId(name, bson.M{dbUserTTLKey: time.Time{}})
	s.Require().NoError(err)
	actualData, err = s.mongoDepot.Get(depot.CrtTag(name))
	s.Error(err)
	s.Nil(actualData)
}

func (s *MongoDepotTestSuite) TestDelete() {
	deleteName := "alice"
	deleteData := []byte("alice's fake certificate")
	deleteU := &DBUser{
		ID:   deleteName,
		Cert: string(deleteData),
		TTL:  time.Now(),
	}
	name := "bob"
	data := []byte("bob's fake certificate")
	u := &DBUser{
		ID:   name,
		Cert: string(data),
		TTL:  time.Now(),
	}

	// delete does not return an error when user does not exist
	s.NoError(s.mongoDepot.Delete(depot.CrtTag(deleteName)))

	// delete removes correct user
	s.Require().NoError(s.collection.Insert(deleteU))
	s.Require().NoError(s.collection.Insert(u))
	s.NoError(s.mongoDepot.Delete(depot.CrtTag(deleteName)))
	deleteU = &DBUser{}
	s.Equal(mgo.ErrNotFound, s.collection.FindId(deleteName).One(deleteU))
	u = &DBUser{}
	s.Require().NoError(s.collection.FindId(name).One(u))
	s.Equal(name, u.ID)
}
