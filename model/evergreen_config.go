package model

import "github.com/mongodb/anser/bsonutil"

// EvergreenConnectionInfo stores the root URL, username, and API key for the user
type EvergreenConnectionInfo struct {
	RootURL string `bson:"url" json:"url" yaml:"url"`
	User    string `bson:"user" json:"user" yaml:"user"`
	Key     string `bson:"key" json:"key" yaml:"key"`
}

var (
	evergreenConnInfoRootURLKey = bsonutil.MustHaveTag(EvergreenConnectionInfo{}, "RootURL")
	evergreenConnInfoUserKey    = bsonutil.MustHaveTag(EvergreenConnectionInfo{}, "User")
	evergreenConnInfoKeyKey     = bsonutil.MustHaveTag(EvergreenConnectionInfo{}, "Key")
)

// IsValid checks that a user, API key, and root URL are given in the
// ConnectionInfo structure
func (e *EvergreenConnectionInfo) IsValid() bool {
	if e == nil {
		return false
	}
	if e.RootURL == "" || e.User == "" || e.Key == "" {
		return false
	}
	return true
}
