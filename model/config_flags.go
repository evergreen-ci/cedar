package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

type OperationalFlags struct {
	// DisableCostReportingJob         bool `bson:"disable_cost_reporting" json:"disable_cost_reporting" yaml:"disable_cost_reporting"`
	DisableInternalMetricsReporting bool `bson:"disable_internal_metrics_reporting" json:"disable_internal_metrics_reporting" yaml:"disable_internal_metrics_reporting"`
	DisableSignalProcessing         bool `bson:"disable_signal_processing" json:"disable_signal_processing" yaml:"disable_signal_processing"`
	DisableHistoricalTestData       bool `bson:"disable_historical_test_data" json:"disable_historical_test_data" yaml:"disable_historical_test_data"`

	env cedar.Environment
}

var (
	// opsFlagsDisableCostReporting            = bsonutil.MustHaveTag(OperationalFlags{}, "DisableCostReportingJob")
	opsFlagsDisableInternalMetricsReporting = bsonutil.MustHaveTag(OperationalFlags{}, "DisableInternalMetricsReporting")
	opsFlagsDisableSignalProcessing         = bsonutil.MustHaveTag(OperationalFlags{}, "DisableSignalProcessing")
)

func (f *OperationalFlags) findAndSet(name string, v bool) error {
	switch name {
	// case "disable_cost_reporting":
	//     return f.SetDisableCostReportingJob(v)
	case "disable_internal_metrics_reporting":
		return f.SetDisableInternalMetricsReporting(v)
	case "disable_signal_processing":
		return f.SetDisableSignalProcessing(v)
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

// func (f *OperationalFlags) SetDisableCostReportingJob(v bool) error {
//     if err := f.update(opsFlagsDisableCostReporting, v); err != nil {
//         return errors.WithStack(err)
//     }
//     f.DisableCostReportingJob = v
//     return nil
// }
//
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
