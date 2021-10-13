package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	configurationCollection = "configuration"
	cedarConfigurationID    = "cedar-system-configuration"
)

type CedarConfig struct {
	ID             string                    `bson:"_id" json:"id" yaml:"id"`
	URL            string                    `bson:"url" json:"url" yaml:"url"`
	Evergreen      EvergreenConfig           `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Splunk         send.SplunkConnectionInfo `bson:"splunk" json:"splunk" yaml:"splunk"`
	Slack          SlackConfig               `bson:"slack" json:"slack" yaml:"slack"`
	LDAP           LDAPConfig                `bson:"ldap" json:"ldap" yaml:"ldap"`
	ServiceAuth    ServiceAuthConfig         `bson:"service_auth" json:"service_auth" yaml:"service_auth"`
	NaiveAuth      NaiveAuthConfig           `bson:"naive_auth" json:"naive_auth" yaml:"naive_auth"`
	CA             CAConfig                  `bson:"ca" json:"ca" yaml:"ca"`
	Bucket         BucketConfig              `bson:"bucket" json:"bucket" yaml:"bucket"`
	Flags          OperationalFlags          `bson:"flags" json:"flags" yaml:"flags"`
	Service        ServiceConfig             `bson:"service" json:"service" yaml:"service"`
	ChangeDetector ChangeDetectorConfig      `bson:"change_detector" json:"change_detector" yaml:"change_detector"`

	populated bool
	env       cedar.Environment
}

func NewCedarConfig(env cedar.Environment) *CedarConfig {
	return &CedarConfig{
		ID: cedarConfigurationID,
		Flags: OperationalFlags{
			env: env,
		},
		env:       env,
		populated: true,
	}
}

var (
	cedarConfigurationIDKey             = bsonutil.MustHaveTag(CedarConfig{}, "ID")
	cedarConfigurationEvergreenKey      = bsonutil.MustHaveTag(CedarConfig{}, "Evergreen")
	cedarConfigurationSplunkKey         = bsonutil.MustHaveTag(CedarConfig{}, "Splunk")
	cedarConfigurationSlackKey          = bsonutil.MustHaveTag(CedarConfig{}, "Slack")
	cedarConfigurationLDAPKey           = bsonutil.MustHaveTag(CedarConfig{}, "LDAP")
	cedarConfigurationServiceAuthKey    = bsonutil.MustHaveTag(CedarConfig{}, "ServiceAuth")
	cedarConfigurationNaiveAuthKey      = bsonutil.MustHaveTag(CedarConfig{}, "NaiveAuth")
	cedarConfigurationCAKey             = bsonutil.MustHaveTag(CedarConfig{}, "CA")
	cedarConfigurationFlagsKey          = bsonutil.MustHaveTag(CedarConfig{}, "Flags")
	cedarConfigurationServiceKey        = bsonutil.MustHaveTag(CedarConfig{}, "Service")
	cedarConfigurationChangeDetectorKey = bsonutil.MustHaveTag(CedarConfig{}, "ChangeDetector")
)

type EvergreenConfig struct {
	URL               string `bson:"url" json:"url" yaml:"url"`
	AuthTokenCookie   string `bson:"auth_token_cookie" json:"auth_token_cookie" yaml:"auth_token_cookie"`
	HeaderKeyName     string `bson:"header_key_name" json:"header_key_name" yaml:"header_key_name"`
	HeaderUserName    string `bson:"header_user_name" json:"header_user_name" yaml:"header_user_name"`
	Domain            string `bson:"domain" json:"domain" yaml:"domain"`
	ServiceUserName   string `bson:"service_user_name" json:"service_user_name" yaml:"service_user_name"`
	ServiceUserAPIKey string `bson:"service_user_api_key" json:"service_user_api_key" yaml:"service_user_api_key"`
}

var (
	cedarEvergreenConfigURLKey             = bsonutil.MustHaveTag(EvergreenConfig{}, "URL")
	cedarEvergreenConfigAuthTokenCookieKey = bsonutil.MustHaveTag(EvergreenConfig{}, "AuthTokenCookie")
	cedarEvergreenConfigHeaderKeyName      = bsonutil.MustHaveTag(EvergreenConfig{}, "HeaderKeyName")
	cedarEvergreenConfigHeaderUserName     = bsonutil.MustHaveTag(EvergreenConfig{}, "HeaderUserName")
	cedarEvergreenConfigDomain             = bsonutil.MustHaveTag(EvergreenConfig{}, "Domain")
	cedarEvergreenConfigServiceUserName    = bsonutil.MustHaveTag(EvergreenConfig{}, "ServiceUserName")
	cedarEvergreenConfigServiceUserAPIKey  = bsonutil.MustHaveTag(EvergreenConfig{}, "ServiceUserAPIKey")
)

type ChangeDetectorConfig struct {
	Implementation string `bson:"implementation" json:"implementation" yaml:"implementation"`
	URI            string `bson:"uri" json:"uri" yaml:"uri"`
	User           string `bson:"user" json:"user" yaml:"user"`
	Token          string `bson:"token" json:"token" yaml:"token"`
}

var (
	cedarChangeDetectorConfigURIKey   = bsonutil.MustHaveTag(ChangeDetectorConfig{}, "URI")
	cedarChangeDetectorConfigTokenKey = bsonutil.MustHaveTag(ChangeDetectorConfig{}, "Token")
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
	URL          string `bson:"url" json:"url" yaml:"url"`
	Port         string `bson:"port" json:"port" yaml:"port"`
	UserPath     string `bson:"path" json:"path" yaml:"path"`
	ServicePath  string `bson:"service_path" json:"service_path" yaml:"service_path"`
	UserGroup    string `bson:"user_group" json:"user_group" yaml:"user_group"`
	ServiceGroup string `bson:"service_group" json:"service_group" yaml:"service_group"`
}

var (
	cedarLDAPConfigURLKey          = bsonutil.MustHaveTag(LDAPConfig{}, "URL")
	cedarLDAPConfigPortKey         = bsonutil.MustHaveTag(LDAPConfig{}, "Port")
	cedarLDAPConfigUserPathKey     = bsonutil.MustHaveTag(LDAPConfig{}, "UserPath")
	cedarLDAPConfigServicePathKey  = bsonutil.MustHaveTag(LDAPConfig{}, "ServicePath")
	cedarLDAPConfigGroupKey        = bsonutil.MustHaveTag(LDAPConfig{}, "UserGroup")
	cedarLDAPConfigServiceGroupKey = bsonutil.MustHaveTag(LDAPConfig{}, "ServiceGroup")
)

type ServiceAuthConfig struct {
	Enabled bool `bson:"enabled" json:"enabled" yaml:"enabled"`
}

type NaiveAuthConfig struct {
	AppAuth bool              `bson:"app_auth" json:"app_auth" yaml:"app_auth"`
	Users   []NaiveUserConfig `bson:"users" json:"users" yaml:"users"`
}

var (
	cedarNaiveAuthConfigAppAuthKey = bsonutil.MustHaveTag(NaiveAuthConfig{}, "AppAuth")
	cedarNaiveAuthConfigUsersKey   = bsonutil.MustHaveTag(NaiveAuthConfig{}, "Users")
)

type NaiveUserConfig struct {
	ID           string   `bson:"_id" json:"id" yaml:"id"`
	Name         string   `bson:"name" json:"name" yaml:"name"`
	EmailAddress string   `bson:"email" json:"email" yaml:"email"`
	Password     string   `bson:"password" json:"password" yaml:"password"`
	Key          string   `bson:"key" json:"key" yaml:"key"`
	AccessRoles  []string `bson:"roles" json:"roles" yaml:"roles"`
	Invalid      bool     `bson:"invalid" json:"invalid" yaml:"invalid"`
}

var (
	cedarNaiveUserConfigIDKey           = bsonutil.MustHaveTag(NaiveUserConfig{}, "ID")
	cedarNaiveUserConfigNameKey         = bsonutil.MustHaveTag(NaiveUserConfig{}, "Name")
	cedarNaiveUserConfigEmailAddressKey = bsonutil.MustHaveTag(NaiveUserConfig{}, "EmailAddress")
	cedarNaiveUserConfigPasswordKey     = bsonutil.MustHaveTag(NaiveUserConfig{}, "Password")
	cedarNaiveUserConfigKeyKey          = bsonutil.MustHaveTag(NaiveUserConfig{}, "Key")
	cedarNaiveUserConfigAccessRolesKey  = bsonutil.MustHaveTag(NaiveUserConfig{}, "AccessRoles")
	cedarNaiveUserConfigInvalidKey      = bsonutil.MustHaveTag(NaiveUserConfig{}, "Invalid")
)

type CAConfig struct {
	CertDepot         certdepot.BootstrapDepotConfig `bson:"certdepot" json:"certdepot" yaml:"certdepot"`
	SSLExpireAfter    time.Duration                  `bson:"ssl_expire" json:"ssl_expire" yaml:"ssl_expire"`
	SSLRenewalBefore  time.Duration                  `bson:"ssl_renewal" json:"ssl_renewal" yaml:"ssl_renewal"`
	ServerCertVersion int                            `bson:"server_cert_version"`
}

var (
	cedarCAConfigCertDepotKey        = bsonutil.MustHaveTag(CAConfig{}, "CertDepot")
	cedarCAConfigSSLExpireAfterKey   = bsonutil.MustHaveTag(CAConfig{}, "SSLExpireAfter")
	cedarCAConfigSSLRenewalBeforeKey = bsonutil.MustHaveTag(CAConfig{}, "SSLRenewalBefore")
)

// Credentials and other configuration information for pail Bucket usage.
type BucketConfig struct {
	AWSKey                  string   `bson:"aws_key" json:"aws_key" yaml:"aws_key"`
	AWSSecret               string   `bson:"aws_secret" json:"aws_secret" yaml:"aws_secret"`
	BuildLogsBucket         string   `bson:"build_logs_bucket" json:"build_logs_bucket" yaml:"build_logs_bucket"`
	SystemMetricsBucket     string   `bson:"system_metrics_bucket" json:"system_metrics_bucket" yaml:"system_metrics_bucket"`
	SystemMetricsBucketType PailType `bson:"system_metrics_bucket_type" json:"system_metrics_bucket_type" yaml:"system_metrics_bucket_type"`
	TestResultsBucket       string   `bson:"test_results_bucket" json:"test_results_bucket" yaml:"test_results_bucket"`
	TestResultsBucketType   PailType `bson:"test_results_bucket_type" json:"test_results_bucket_type" yaml:"test_results_bucket_type"`
}

var (
	cedarS3BucketConfigAWSKeyKey          = bsonutil.MustHaveTag(BucketConfig{}, "AWSKey")
	cedarS3BucketConfigAWSSecretKey       = bsonutil.MustHaveTag(BucketConfig{}, "AWSSecret")
	cedarS3BucketConfigBuildLogsBucketKey = bsonutil.MustHaveTag(BucketConfig{}, "BuildLogsBucket")
)

type ServiceConfig struct {
	AppServers  []string `bson:"app_servers" json:"app_servers" yaml:"app_servers"`
	CORSOrigins []string `bson:"cors_origins" json:"cors_origins" yaml:"cors_origins"`
}

func (c *CedarConfig) Setup(e cedar.Environment) { c.env = e }
func (c *CedarConfig) IsNil() bool               { return !c.populated }
func (c *CedarConfig) Find() error {
	if c.env == nil {
		return errors.New("cannot find configuration with a nil environment")
	}

	if value, ok := c.env.GetCachedDBValue(cedarConfigurationID); ok {
		switch t := value.(type) {
		case CedarConfig:
			*c = t
			return nil
		default:
			return errors.Errorf("unrecognized cached cedar config type '%v'", t)
		}
	}

	if err := c.find(); err != nil {
		return err
	}

	updateChan := make(chan interface{})
	if closeChan, ok := c.env.RegisterDBValueCacher(cedarConfigurationID, *c, updateChan); ok {
		go c.changeStream(updateChan, closeChan)
	}

	return nil
}

func (c *CedarConfig) find() error {
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
		return errors.Wrap(err, "finding app config document")
	}

	c.populated = true
	c.Flags.env = c.env

	return nil
}

func (c *CedarConfig) changeStream(updateChan chan interface{}, closeChan chan struct{}) {
	defer recovery.LogStackTraceAndContinue("cedar config updater")

	ctx, cancel := c.env.Context()
	defer cancel()

	var err error
	defer func() {
		if err == nil {
			return
		}

		select {
		case updateChan <- err:
		case <-ctx.Done():
		}
	}()

	stream, err := c.env.GetDB().Collection(configurationCollection).Watch(ctx, bson.D{})
	if err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message": "getting configuration collection change stream",
		}))
		return
	}
	defer func() {
		if err = stream.Close(nil); err != nil {
			grip.Error(message.WrapError(err, message.Fields{
				"message": "closing configuration collection change stream",
			}))
		}
	}()

	for stream.Next(ctx) {
		updatedConf := &CedarConfig{env: c.env}
		if err = updatedConf.find(); err != nil {
			grip.Error(message.WrapError(err, message.Fields{
				"message": "getting updated configuration",
			}))
			return
		}

		select {
		case updateChan <- *updatedConf:
		case <-closeChan:
			return
		case <-ctx.Done():
			return
		}
	}

	if err = stream.Err(); err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message": "configuration collection change stream error",
		}))
	}
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
		"operation":   "save application configuration",
		"change_info": changeInfo,
	})

	return errors.Wrap(err, "problem saving application configuration")
}

func LoadCedarConfig(file string) (*CedarConfig, error) {
	newConfig := &CedarConfig{}

	if err := utility.ReadYAMLFile(file, newConfig); err != nil {
		return nil, errors.WithStack(err)
	}

	// TODO: validate here (?)

	newConfig.populated = true

	return newConfig, nil
}
