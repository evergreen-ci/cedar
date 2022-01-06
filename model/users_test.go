package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/db"
	"github.com/stretchr/testify/suite"
)

type UserTestSuite struct {
	users []*User
	sess  db.Session
	c     db.Collection
	suite.Suite
}

func TestUser(t *testing.T) {
	suite.Run(t, &UserTestSuite{})
}

func (s *UserTestSuite) SetupTest() {
	env := cedar.GetEnvironment()
	conf, session, err := cedar.GetSessionWithConfig(env)
	s.Require().NoError(err)
	s.sess = session
	s.c = session.DB(conf.DatabaseName).C(userCollection)
	_ = s.c.DropCollection()

	s.users = []*User{
		{
			ID: "Test1",
			LoginCache: LoginCache{
				Token: "1234",
				TTL:   time.Now(),
			},
		},
		{
			ID: "Test2",
			LoginCache: LoginCache{
				Token: "4321",
				TTL:   time.Now().Add(-time.Hour),
			},
		},
		{
			ID: "Test3",
			LoginCache: LoginCache{
				Token: "5678",
				TTL:   time.Now(),
			},
		},
	}

	for _, user := range s.users {
		s.Require().NoError(s.c.Insert(user))
		user.Setup(env)
	}
}

func (s *UserTestSuite) TearDownTest() {
	s.Require().NoError(s.c.DropCollection())
	s.sess.Close()
}

func (s *UserTestSuite) TestGetUser() {
	u, valid, err := GetUser(s.users[0].ID)
	s.NoError(err)
	s.True(valid)
	s.Require().NotNil(u)
	s.Equal("Test1", u.Username())

	u, valid, err = GetUser(s.users[1].ID)
	s.NoError(err)
	s.False(valid)
	s.Require().NotNil(u)
	s.Equal("Test2", u.Username())

	u, valid, err = GetUser("DNE")
	s.Error(err)
	s.False(valid)
	s.Nil(u)
}

func (s *UserTestSuite) TestGetOrAddUser() {
	u, err := GetOrAddUser(s.users[0])
	s.Require().NoError(err)
	s.NotNil(u)
	s.Equal("Test1", u.Username())

	u, err = GetOrAddUser(s.users[1])
	s.NoError(err)
	s.NotNil(u)
	s.Equal("Test2", u.Username())

	newUser := &User{
		ID:           "NewUser",
		Display:      "user",
		EmailAddress: "fake@fake.com",
		APIKey:       "2345435",
		SystemRoles:  []string{"admin"},
	}
	u, err = GetOrAddUser(newUser)
	s.NoError(err)
	s.Equal("NewUser", u.Username())
	s.Equal("user", u.DisplayName())
	s.Equal("fake@fake.com", u.Email())
	s.Equal("2345435", u.GetAPIKey())
	s.Equal([]string{"admin"}, u.Roles())
	fromDB := &User{}
	s.Require().NoError((s.c.FindId("NewUser").One(fromDB)))
	s.Equal("NewUser", fromDB.ID)
	s.Equal("user", fromDB.Display)
	s.Equal("fake@fake.com", fromDB.EmailAddress)
	s.Equal("2345435", fromDB.APIKey)
	s.Equal([]string{"admin"}, fromDB.SystemRoles)
	s.NotEmpty(fromDB.LoginCache.Token)
	s.NotEqual(time.Time{}, fromDB.LoginCache.TTL)
}

func (s *UserTestSuite) TestPutLoginCache() {
	token1, err := PutLoginCache(s.users[0])
	s.NoError(err)
	s.NotEmpty(token1)

	token2, err := PutLoginCache(s.users[1])
	s.NoError(err)
	s.NotEmpty(token2)

	token3, err := PutLoginCache(&User{ID: "asdf"})
	s.Error(err)
	s.Empty(token3)

	u1 := &User{}
	s.Require().NoError(s.c.FindId(s.users[0].ID).One(u1))
	s.Equal(s.users[0].ID, u1.ID)

	u2 := &User{}
	s.Require().NoError(s.c.FindId(s.users[1].ID).One(u2))
	s.Equal(s.users[1].ID, u2.ID)

	s.NotEqual(u1.LoginCache.Token, u2.LoginCache.Token)
	s.WithinDuration(time.Now(), u1.LoginCache.TTL, time.Second)
	s.WithinDuration(time.Now(), u2.LoginCache.TTL, time.Second)

	// Put to first user again, ensuring token stays the same but TTL changes
	time.Sleep(time.Millisecond) // sleep to check TTL changed
	token4, err := PutLoginCache(s.users[0])
	s.NoError(err)
	u1Updated := &User{}
	s.Require().NoError(s.c.FindId(s.users[0].ID).One(u1Updated))
	s.Equal(u1.LoginCache.Token, u1Updated.LoginCache.Token)
	s.NotEqual(u1.LoginCache.TTL, u1Updated.LoginCache.TTL)
	s.Equal(token1, token4)

	// Fresh user with no token should generate new token
	token5, err := PutLoginCache(s.users[2])
	s.NoError(err)
	u5 := &User{}
	s.Require().NoError(s.c.FindId(s.users[2].ID).One(u5))
	s.Equal(token5, u5.LoginCache.Token)
	s.NoError(err)
	s.NotEmpty(token5)
	s.NotEqual(token1, token5)
	s.NotEqual(token2, token5)
	s.NotEqual(token3, token5)
	s.NotEqual(token4, token5)
}

func (s *UserTestSuite) TestGetLoginCache() {
	u, valid, err := GetLoginCache("1234")
	s.NoError(err)
	s.True(valid)
	s.Require().NotNil(u)
	s.Equal("Test1", u.Username())

	u, valid, err = GetLoginCache("4321")
	s.NoError(err)
	s.False(valid)
	s.Require().NotNil(u)
	s.Equal("Test2", u.Username())

	u, valid, err = GetLoginCache("asdf")
	s.NoError(err)
	s.False(valid)
	s.Nil(u)
}

func (s *UserTestSuite) TestClearLoginCacheSingleUser() {
	// Error on non-existent user
	s.Error(ClearLoginCache(&User{ID: "asdf"}, false))

	// Two valid users...
	u1, valid, err := GetLoginCache("1234")
	s.Require().NoError(err)
	s.Require().True(valid)
	s.Require().Equal("Test1", u1.Username())
	u2, valid, err := GetLoginCache("5678")
	s.Require().NoError(err)
	s.Require().True(valid)
	s.Require().Equal("Test3", u2.Username())

	// One is cleared...
	s.NoError(ClearLoginCache(u1, false))
	// and is no longer found
	u1, valid, err = GetLoginCache("1234")
	s.NoError(err)
	s.False(valid)
	s.Nil(u1)

	// The other user remains
	u2, valid, err = GetLoginCache("5678")
	s.NoError(err)
	s.True(valid)
	s.Equal("Test3", u2.Username())
}

func (s *UserTestSuite) TestClearLoginCacheAllUsers() {
	// Clear all users
	s.NoError(ClearLoginCache(nil, true))
	// Sample users no longer in cache
	for _, token := range []string{"1234", "4321", "5678", "expired"} {
		u, valid, err := GetLoginCache(token)
		s.NoError(err)
		s.False(valid)
		s.Nil(u)
	}
}
