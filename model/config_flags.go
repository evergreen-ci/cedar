package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

type OperationalFlags struct {
	DisableInternalMetricsReporting bool `bson:"disable_internal_metrics_reporting" json:"disable_internal_metrics_reporting" yaml:"disable_internal_metrics_reporting"`
	DisableSignalProcessing         bool `bson:"disable_signal_processing" json:"disable_signal_processing" yaml:"disable_signal_processing"`
	DisableHistoricalTestData       bool `bson:"disable_historical_test_data" json:"disable_historical_test_data" yaml:"disable_historical_test_data"`
	DisableAmboyRetries             bool `bson:"disable_amboy_retries" json:"disable_amboy_retries" yaml:"disable_amboy_retries"`

	env cedar.Environment
}

var (
	opsFlagsDisableInternalMetricsReporting = bsonutil.MustHaveTag(OperationalFlags{}, "DisableInternalMetricsReporting")
	opsFlagsDisableSignalProcessing         = bsonutil.MustHaveTag(OperationalFlags{}, "DisableSignalProcessing")
	opsFlagsDisableHistoricalTestData       = bsonutil.MustHaveTag(OperationalFlags{}, "DisableHistoricalTestData")
	opsFlagsDisableAmboyRetries             = bsonutil.MustHaveTag(OperationalFlags{}, "DisableAmboyRetries")
)

func (f *OperationalFlags) findAndSet(name string, v bool) error {
	switch name {
	case opsFlagsDisableInternalMetricsReporting:
		return f.SetDisableInternalMetricsReporting(v)
	case opsFlagsDisableSignalProcessing:
		return f.SetDisableSignalProcessing(v)
	case opsFlagsDisableAmboyRetries:
		return f.SetDisableAmboyRetries(v)
	case opsFlagsDisableHistoricalTestData:
		return f.SetDisableHistoricalTestData(v)
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

func (f *OperationalFlags) SetDisableInternalMetricsReporting(v bool) error {
	if err := f.update(opsFlagsDisableInternalMetricsReporting, v); err != nil {
		return errors.WithStack(err)
	}
	f.DisableInternalMetricsReporting = v
	return nil

}

func (f *OperationalFlags) SetDisableSignalProcessing(v bool) error {
	if err := f.update(opsFlagsDisableSignalProcessing, v); err != nil {
		return errors.WithStack(err)
	}
	f.DisableSignalProcessing = v
	return nil

}

func (f *OperationalFlags) SetDisableHistoricalTestData(v bool) error {
	if err := f.update(opsFlagsDisableHistoricalTestData, v); err != nil {
		return errors.WithStack(err)
	}
	f.DisableHistoricalTestData = v
	return nil
}

func (f *OperationalFlags) SetDisableAmboyRetries(v bool) error {
	if err := f.update(opsFlagsDisableAmboyRetries, v); err != nil {
		return errors.WithStack(err)
	}
	f.DisableAmboyRetries = v
	return nil
}

func (f *OperationalFlags) update(key string, value bool) error {
	conf, session, err := cedar.GetSessionWithConfig(f.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	err = session.DB(conf.DatabaseName).C(configurationCollection).UpdateId(cedarConfigurationID, bson.M{"$set": bson.M{key: value}})
	if err != nil {
		return errors.Wrapf(err, "problem setting %s to %t", key, value)
	}

	return nil
}
