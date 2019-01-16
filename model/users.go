package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

const userCollection = "users"

// Stores user information in database, resulting in a cache for the LDAP user manager.
type DBUser struct {
	Id           string     `bson:"_id"`
	Display      string     `bson:"display_name"`
	EmailAddress string     `bson:"email"`
	CreatedAt    time.Time  `bson:"created_at"`
	APIKey       string     `bson:"apikey"`
	SystemRoles  []string   `bson:"roles"`
	LoginCache   LoginCache `bson:"login_cache"`
}

var (
	dbUserIdKey           = bsonutil.MustHaveTag(DBUser{}, "Id")
	dbUserDisplayNameKey  = bsonutil.MustHaveTag(DBUser{}, "Display")
	dbUserEmailAddressKey = bsonutil.MustHaveTag(DBUser{}, "EmailAddress")
	dbUserAPIKeyKey       = bsonutil.MustHaveTag(DBUser{}, "APIKey")
	dbUserSystemRolesKey  = bsonutil.MustHaveTag(DBUser{}, "SystemRoles")
	dbUserLoginCacheKey   = bsonutil.MustHaveTag(DBUser{}, "LoginCache")
)

type LoginCache struct {
	Token string    `bson:"token"`
	TTL   time.Time `bson:"ttl"`
}

var (
	loginCacheTokenKey = bsonutil.MustHaveTag(LoginCache{}, "Token")
	loginCacheTTLKey   = bsonutil.MustHaveTag(LoginCache{}, "TTL")
)

func (u *DBUser) Email() string     { return u.EmailAddress }
func (u *DBUser) Username() string  { return u.Id }
func (u *DBUser) GetAPIKey() string { return u.APIKey }
func (u *DBUser) Roles() []string   { return u.SystemRoles }

func (u *DBUser) DisplayName() string {
	if u.Display != "" {
		return u.Display
	}
	return u.Id
}

// PutLoginCache generates, saves, and returns a new token; the user's TTL is
// updated.
func PutLoginCache(user gimlet.User) (string, error) {
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer session.Close()

	u := &DBUser{}
	coll := session.DB(conf.DatabaseName).C(userCollection)
	err = coll.FindId(user.Username()).One(u)
	if db.ResultsNotFound(err) {
		return "", errors.Errorf("could not find user %s in the database", user.Username())
	} else if err != nil {
		return "", errors.Wrap(err, "problem finding user")
	}

	var update bson.M
	token := u.LoginCache.Token
	// TODO: figure out if we should reset token if TTL has expired
	if token == "" {
		token = util.RandomString()
		update = bson.M{"$set": bson.M{
			bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey): token,
			bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTTLKey):   time.Now(),
		}}
	} else {
		update = bson.M{"$set": bson.M{
			bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTTLKey): time.Now(),
		}}
	}
	if err = coll.Update(bson.M{dbUserIdKey: u.Id}, update); err != nil {
		return "", errors.Wrap(err, "problem updating user cache")
	}

	return token, nil
}

// GetUserByToken retrieves cached users by token.
// It returns an error if and only if there was an error retrieving the user
// from the cache.
// It returns (<user>, true, nil) if the user is present in the cache and is
// valid.
// It returns (<user>, false, nil) if the user is present in the cache but has
// expired.
// It returns (nil, false, nil) if the user is not present in the cache.
func GetLoginCache(token string) (gimlet.User, bool, error) {
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
	if err != nil {
		return nil, false, errors.WithStack(err)
	}
	defer session.Close()

	user := &DBUser{}
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
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
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
		u := &DBUser{}
		err = session.DB(conf.DatabaseName).C(userCollection).FindId(user.Username()).One(u)
		if db.ResultsNotFound(err) {
			return errors.Errorf("could not find user %s in the database", user.Username())
		} else if err != nil {
			return errors.Wrapf(err, "problem finding user %s by id", user.Username())
		}
		query := bson.M{dbUserIdKey: u.Id}
		if err = session.DB(conf.DatabaseName).C(userCollection).Update(query, update); err != nil {
			return errors.Wrap(err, "problem updating user cache")
		}
	}
	return nil
}

// GetUser gets a user by id from persistent storage.
func GetUser(id string) (gimlet.User, error) {
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer session.Close()

	user := &DBUser{}
	err = session.DB(conf.DatabaseName).C(userCollection).FindId(id).One(user)
	if db.ResultsNotFound(err) {
		return nil, errors.Errorf("could not find user %s in the database", id)
	} else if err != nil {
		return nil, errors.Wrapf(err, "problem finding user %s by id", id)
	}

	return user, nil
}

// GetOrAddUser gets a user from persistent storage, or if the user does not
// exist, to create and save it.
func GetOrAddUser(user gimlet.User) (gimlet.User, error) {
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer session.Close()

	u := &DBUser{}
	err = session.DB(conf.DatabaseName).C(userCollection).FindId(user.Username()).One(u)
	if db.ResultsNotFound(err) {
		u.Id = user.Username()
		u.Display = user.DisplayName()
		u.EmailAddress = user.Email()
		u.APIKey = user.GetAPIKey()
		u.SystemRoles = user.Roles()
		u.CreatedAt = time.Now()
		u.LoginCache = LoginCache{Token: util.RandomString(), TTL: time.Now()}

		err = session.DB(conf.DatabaseName).C(userCollection).Insert(u)
		if err != nil {
			return nil, errors.Wrapf(err, "problem inserting user %s", user.Username())
		}
	} else if err != nil {
		return nil, errors.Wrapf(err, "problem finding user %s by id", user.Username())
	}

	return u, nil
}
