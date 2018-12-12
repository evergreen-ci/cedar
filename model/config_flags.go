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

func (f *OperationalFlags) findAndSet(name string, v bool) error {
	switch name {
	case "disable_cost_reporting":
		return f.SetDisableCostReportingJob(v)
	default:
		return errors.Errorf("%s is not a known feature flag name", name)
	}
}

func (f *OperationalFlags) SetTrue(name string) error {
	return errors.WithStack(f.findAndSet(name, true))
}

func (f *OperationalFlags) SetFalse(name string) error {
	return errors.WithStack(f.findAndSet(name, false))
}

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
