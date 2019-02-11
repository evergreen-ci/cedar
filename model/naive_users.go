package model

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"

	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

type NaiveUser struct {
	User         string `bson:"username" json:"username" yaml:"username"`
	Pass         string `bson:"password" json:"password" yaml:"password"`
	EmailAddress string `bson:"email" json:"email" yaml:"email"`
}

func (u *NaiveUser) DisplayName() string { return u.User }
func (u *NaiveUser) Email() string       { return u.Pass }
func (u *NaiveUser) Username() string    { return u.User }
func (u *NaiveUser) GetAPIKey() string   { return "" }
func (u *NaiveUser) Roles() []string     { return []string{} }

// NaiveUserManager implements the UserManager interface and has a list of
// NaiveUsers which is stored in the settings configuration file.
// Note: This use of the UserManager is recommended for dev/test purposes only
// and users who need high security authentication  mechanisms should rely on a
// different authentication mechanism.
type NaiveUserManager struct {
	users []*NaiveUser
}

func NewNaiveUserManager(naiveAuthConfig *NaiveAuthConfig) (gimlet.UserManager, error) {
	users := naiveAuthConfig.Users
	return &NaiveUserManager{users}, nil
}

// GetUserByToken does a find by creating a temporary token from the index of
// the user on the list, the email of the user and a hash of the username and
// password, checking it against the token string and returning a User if
// there is a match.
func (um *NaiveUserManager) GetUserByToken(_ context.Context, token string) (gimlet.User, error) {
	for i, user := range um.users {
		//check to see if token exists
		possibleToken := fmt.Sprintf("%v:%v:%v", i, user.Email, md5.Sum([]byte(user.User+user.Pass)))
		if token == possibleToken {
			return user, nil
		}
	}
	return nil, errors.New("No valid user found")
}

// CreateUserToken finds the user with the same username and password in its
// list of users and creates a token that is a combination of the index of the
// list the user is at, the email address and a hash of the username and
// password and returns that token.
func (um *NaiveUserManager) CreateUserToken(username, password string) (string, error) {
	for i, user := range um.users {
		if user.User == username && user.Pass == password {
			// return a token that is a hash of the index, user's email and username and password hashed.
			return fmt.Sprintf("%v:%v:%v", i, user.Email, md5.Sum([]byte(user.User+user.Pass))), nil
		}
	}
	return "", errors.New("No valid user for the given username and password")
}

func (*NaiveUserManager) GetLoginHandler(string) http.HandlerFunc   { return nil }
func (*NaiveUserManager) GetLoginCallbackHandler() http.HandlerFunc { return nil }
func (*NaiveUserManager) IsRedirect() bool                          { return false }

func (um *NaiveUserManager) GetUserByID(id string) (gimlet.User, error) {
	for _, user := range um.users {
		if user.User == id {
			return user, nil
		}
	}
	return nil, errors.Errorf("user %s not found!", id)
}

func (um *NaiveUserManager) GetOrCreateUser(u gimlet.User) (gimlet.User, error) {
	existingUser, err := um.GetUserByID(u.Username())
	if err == nil {
		return existingUser, nil
	}

	newUser := &NaiveUser{
		User:         u.Username(),
		EmailAddress: u.Email(),
	}
	um.users = append(um.users, newUser)
	return newUser, nil
}

func (b *NaiveUserManager) ClearUser(u gimlet.User, all bool) error {
	return errors.New("Naive Authentication does not support Clear User")
}
