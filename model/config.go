package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
)

const cedarConfigurationID = "cedar-system-configuration"

type CedarConfig struct {
	ID     string                    `bson:"_id" json:"id" yaml:"id"`
	Splunk send.SplunkConnectionInfo `bson:"splunk" json:"splunk" yaml:"splunk"`
	Slack  SlackConfig               `bson:"slack" json:"slack" yaml:"slack"`
	Auth   LDAPConfig                `bson:"auth" json:'auth" yaml"auth"`
	Flags  OperationalFlags          `bson:"flags" json:"flags" yaml:"flags"`

	populated bool
	env       cedar.Environment
}

func NewCedarConfig(env cedar.Environment) *CedarConfig {
	return &CedarConfig{
		ID:  cedarConfigurationID,
		env: env,
		Flags: OperationalFlags{
			env: env,
		},
	}
}

var (
	cedarConfigurationIDKey     = bsonutil.MustHaveTag(CedarConfig{}, "ID")
	cedarConfigurationSplunkKey = bsonutil.MustHaveTag(CedarConfig{}, "Splunk")
	cedarConfigurationSlackKey  = bsonutil.MustHaveTag(CedarConfig{}, "Slack")
	cedarConfigurationAuthKey   = bsonutil.MustHaveTag(CedarConfig{}, "Auth")
	cedarConfigurationFlagsKey  = bsonutil.MustHaveTag(CedarConfig{}, "Flags")
)

type SlackConfig struct {
	Options *send.SlackOptions `bson:"options" json:"options" yaml:"options"`
	Token   string             `bson:"token" json:"token" yaml:"token"`
	Level   string             `bson:"level" json:"level" yaml:"level"`
}

var (
	cedarSlackConfigOptionsKey = bsonutil.MustHaveTag(SlackConfig{}, "Options")
	cedarSlackConfigTokenKey   = bsonutil.MustHaveTag(SlackConfig{}, "Token")
	cedarSlackConfigLevelKey   = bsonutil.MustHaveTag(SlackConfig{}, "Level")
)

// LDAPConfig contains settings for interacting with an LDAP server.
type LDAPConfig struct {
	URL                string `bson:"url" json:"url" yaml:"url"`
	Port               string `bson:"port" json:"port" yaml:"port"`
	UserPath           string `bson:"path" json:"path" yaml:"path"`
	ServicePath        string `bson:"service_path" json:"service_path" yaml:"service_path"`
	UserGroup          string `bson:"user_group" json:"user_group" yaml:"user_group"`
	ServiceGroup       string `bson:"service_group" json:"service_group" yaml:"service_group"`
	ExpireAfterMinutes string `bson:"expire_after_minutes" json:"expire_after_minutes" yaml:"expire_after_minutes"`
}

var (
	cedarLDAPConfigURLKey                = bsonutil.MustHaveTag(LDAPConfig{}, "URL")
	cedarLDAPConfigPortKey               = bsonutil.MustHaveTag(LDAPConfig{}, "Port")
	cedarLDAPConfigUserPathKey           = bsonutil.MustHaveTag(LDAPConfig{}, "UserPath")
	cedarLDAPConfigServicePathKey        = bsonutil.MustHaveTag(LDAPConfig{}, "ServicePath")
	cedarLDAPConfigGroupKey              = bsonutil.MustHaveTag(LDAPConfig{}, "UserGroup")
	cedarLDAPConfigServiceGroupKey       = bsonutil.MustHaveTag(LDAPConfig{}, "ServiceGroup")
	cedarLDAPConfigExpireAfterMinutesKey = bsonutil.MustHaveTag(LDAPConfig{}, "ExpireAfterMinutes")
)

func (c *CedarConfig) Setup(e cedar.Environment) { c.env = e }
func (c *CedarConfig) IsNil() bool               { return !c.populated }
func (c *CedarConfig) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	c.populated = false
	err = session.DB(conf.DatabaseName).C(configurationCollection).FindId(cedarConfigurationID).One(c)
	if db.ResultsNotFound(err) {
		return errors.New("could not find application configuration in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding app config document")
	}

	c.populated = true
	c.Flags.env = c.env

	return nil
}

func (c *CedarConfig) Save() error {
	// TODO: validate here when that's possible

	if !c.populated {
		return errors.New("cannot save a non-populated app configuration")
	}

	c.ID = cedarConfigurationID

	conf, session, err := cedar.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(configurationCollection).UpsertId(cedarConfigurationID, c)
	grip.Debug(message.Fields{
		"ns":          model.Namespace{DB: conf.DatabaseName, Collection: configurationCollection},
		"id":          cedarConfigurationID,
		"operation":   "save build cost reporting configuration",
		"change_info": changeInfo,
	})

	return errors.Wrap(err, "problem saving application configuration")
}

func LoadCedarConfig(file string) (*CedarConfig, error) {
	newConfig := &CedarConfig{}

	if err := util.ReadFileYAML(file, newConfig); err != nil {
		return nil, errors.WithStack(err)
	}

	// TODO: validate here (?)

	newConfig.populated = true

	return newConfig, nil
}
