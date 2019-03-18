package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const userCollection = "users"

// Stores user information in database, resulting in a cache for the LDAP user manager.
type User struct {
	ID           string     `bson:"_id"`
	Display      string     `bson:"display_name"`
	EmailAddress string     `bson:"email"`
	CreatedAt    time.Time  `bson:"created_at"`
	APIKey       string     `bson:"apikey"`
	SystemRoles  []string   `bson:"roles"`
	LoginCache   LoginCache `bson:"login_cache"`

	env       cedar.Environment
	populated bool
}

var (
	dbUserIDKey           = bsonutil.MustHaveTag(User{}, "ID")
	dbUserDisplayNameKey  = bsonutil.MustHaveTag(User{}, "Display")
	dbUserEmailAddressKey = bsonutil.MustHaveTag(User{}, "EmailAddress")
	dbUserAPIKeyKey       = bsonutil.MustHaveTag(User{}, "APIKey")
	dbUserSystemRolesKey  = bsonutil.MustHaveTag(User{}, "SystemRoles")
	dbUserLoginCacheKey   = bsonutil.MustHaveTag(User{}, "LoginCache")
)

type LoginCache struct {
	Token string    `bson:"token"`
	TTL   time.Time `bson:"ttl"`
}

var (
	loginCacheTokenKey = bsonutil.MustHaveTag(LoginCache{}, "Token")
	loginCacheTTLKey   = bsonutil.MustHaveTag(LoginCache{}, "TTL")
)

func (u *User) Setup(env cedar.Environment) { u.env = env }
func (u *User) IsNil() bool                 { return !u.populated }
func (u *User) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(u.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	u.populated = false
	err = session.DB(conf.DatabaseName).C(userCollection).FindId(u.ID).One(u)
	if db.ResultsNotFound(err) {
		return errors.Wrapf(err, "could not find user %s in the database", u.Username())
	} else if err != nil {
		return errors.Wrap(err, "problem finding user")
	}

	u.populated = true
	return nil
}

func (u *User) Save() error {
	if !u.populated {
		return errors.New("cannot save unpopulated document")
	}

	conf, session, err := cedar.GetSessionWithConfig(u.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	_, err = session.DB(conf.DatabaseName).C(userCollection).UpsertId(u.ID, u)
	return errors.WithStack(err)
}

func (u *User) Email() string     { return u.EmailAddress }
func (u *User) Username() string  { return u.ID }
func (u *User) GetAPIKey() string { return u.APIKey }
func (u *User) Roles() []string   { return u.SystemRoles }

func (u *User) DisplayName() string {
	if u.Display != "" {
		return u.Display
	}
	return u.ID
}

func (u *User) SetAPIKey() (string, error) {
	conf, session, err := cedar.GetSessionWithConfig(u.env)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer session.Close()

	k := util.RandomString()

	err = session.DB(conf.DatabaseName).C(userCollection).UpdateId(u.ID, bson.M{
		dbUserAPIKeyKey: k,
	})

	if db.ResultsNotFound(err) {
		return "", errors.New("could not find user in the database")
	} else if err != nil {
		return "", errors.Wrap(err, "problem updating user key document")
	}

	u.APIKey = k
	return k, nil
}

func (u *User) UpdateLoginCache() (string, error) {
	conf, session, err := cedar.GetSessionWithConfig(u.env)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer session.Close()

	var update bson.M

	if u.LoginCache.Token == "" {
		u.LoginCache.Token = util.RandomString()

		update = bson.M{"$set": bson.M{
			bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey): u.LoginCache.Token,
			bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTTLKey):   time.Now(),
		}}
	} else {
		update = bson.M{"$set": bson.M{
			bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTTLKey): time.Now(),
		}}
	}

	if err = session.DB(conf.DatabaseName).C(userCollection).UpdateId(u.ID, update); err != nil {
		return "", errors.Wrap(err, "problem updating user cache")
	}

	return u.LoginCache.Token, nil
}

// PutLoginCache generates, saves, and returns a new token; the user's TTL is
// updated.
func PutLoginCache(user gimlet.User) (string, error) {
	env := cedar.GetEnvironment()

	u := &User{ID: user.Username()}
	u.Setup(env)
	err := u.Find()

	if db.ResultsNotFound(errors.Cause(err)) {
		return "", errors.Errorf("could not find user %s in the database", user.Username())
	} else if err != nil {
		return "", errors.Wrap(err, "problem finding user")
	}

	u.Setup(env)

	token, err := u.UpdateLoginCache()
	if err != nil {
		return "", errors.WithStack(err)
	}

	return token, nil
}

// GetUserLoginCache retrieves cached users by token.
//
// It returns an error if and only if there was an error retrieving the user
// from the cache.
//
// It returns (<user>, true, nil) if the user is present in the cache and is
// valid.
//
// It returns (<user>, false, nil) if the user is present in the cache but has
// expired.
//
// It returns (nil, false, nil) if the user is not present in the cache.
func GetLoginCache(token string) (gimlet.User, bool, error) {
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
	if err != nil {
		return nil, false, errors.WithStack(err)
	}
	defer session.Close()

	user := &User{}
	query := bson.M{bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey): token}
	err = session.DB(conf.DatabaseName).C(userCollection).Find(query).One(user)
	if db.ResultsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, errors.Wrap(err, "problem getting user from cache")
	}
	if time.Since(user.LoginCache.TTL) > cedar.TokenExpireAfter {
		return user, false, nil
	}
	return user, true, nil
}

// ClearLoginCache removes users' tokens from cache. Passing true will ignore
// the user passed and clear all users.
func ClearLoginCache(user gimlet.User, all bool) error {
	env := cedar.GetEnvironment()

	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	update := bson.M{"$unset": bson.M{dbUserLoginCacheKey: 1}}
	if all {
		query := bson.M{}
		_, err = session.DB(conf.DatabaseName).C(userCollection).UpdateAll(query, update)
		if err != nil {
			return errors.Wrap(err, "problem updating user cache")
		}
	} else {
		u := &User{ID: user.Username()}
		u.Setup(env)
		if err := u.Find(); err != nil {
			return errors.WithStack(err)
		}

		if err := session.DB(conf.DatabaseName).C(userCollection).UpdateId(u.ID, update); err != nil {
			return errors.Wrap(err, "problem updating user cache")
		}
	}
	return nil
}

// GetUser gets a user by id from persistent storage, and returns whether the
// returned user's token is valid or not.
func GetUser(id string) (gimlet.User, bool, error) {
	env := cedar.GetEnvironment()

	u := &User{ID: id}
	u.Setup(env)
	if err := u.Find(); err != nil {
		return nil, false, errors.WithStack(err)
	}

	return u, time.Since(u.LoginCache.TTL) < cedar.TokenExpireAfter, nil
}

// GetOrAddUser gets a user from persistent storage, or if the user does not
// exist, to create and save it.
func GetOrAddUser(user gimlet.User) (gimlet.User, error) {
	env := cedar.GetEnvironment()

	u := &User{ID: user.Username()}
	u.Setup(env)
	err := u.Find()
	if db.ResultsNotFound(errors.Cause(err)) {
		u.ID = user.Username()
		u.Display = user.DisplayName()
		u.EmailAddress = user.Email()
		u.APIKey = user.GetAPIKey()
		u.SystemRoles = user.Roles()
		u.CreatedAt = time.Now()
		u.LoginCache = LoginCache{Token: util.RandomString(), TTL: time.Now()}
		u.populated = true
		if err = u.Save(); err != nil {
			return nil, errors.Wrapf(err, "problem inserting user %s", user.Username())
		}
	} else if err != nil {
		return nil, errors.Wrapf(err, "problem finding user %s by id", user.Username())
	}

	return u, nil
}
