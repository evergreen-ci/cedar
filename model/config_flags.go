package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

type OperationalFlags struct {
	DisableCostReportingJob bool `bson:"disable_cost_reporting" json:"disable_cost_reporting" yaml:"disable_cost_reporting"`

	env cedar.Environment
}

var (
	opsFlagsDisableCostReporting = bsonutil.MustHaveTag(OperationalFlags{}, "DisableCostReportingJob")
)

func (f *OperationalFlags) SetDisableCostReportingJob(v bool) error {
	if err := f.update(opsFlagsDisableCostReporting, v); err != nil {
		return errors.WithStack(err)
	}
	f.DisableCostReportingJob = v
	return nil
}

func (f *OperationalFlags) update(key string, value bool) error {
	conf, session, err := cedar.GetSessionWithConfig(f.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	grip.Debug(message.Fields{
		"key_name":  key,
		"new_value": value,
		"old_value": f,
	})

	err = session.DB(conf.DatabaseName).C(configurationCollection).UpdateId(cedarConfigurationID, bson.M{"$set": bson.M{key: value}})
	if err != nil {
		return errors.Wrapf(err, "problem setting %s to %t", key, value)
	}

	return nil
}
