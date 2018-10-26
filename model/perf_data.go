package model

import (
	"errors"

	legacyBSON "gopkg.in/mgo.v2/bson"
)

type PerfRollupType int

const (
	RollupInvalidType PerfRollupType = iota
	RollupFloat
	RollupInt
	RollupIntLong
)

type PerfRollupValue struct {
	Value interface{}
	Type  PerfRollupType
}

func (v PerfRollupValue) Int32() (int32, error) {
	if v.Type != RollupInt {
		return 0, errors.New("mismatched types")
	}
	return v.Value.(int32), nil
}

func (v PerfRollupValue) Int() (int, error) {
	if v.Type != RollupInt {
		return 0, errors.New("mismatched types")
	}
	return v.Value.(int), nil
}

func (v PerfRollupValue) Int64() (int64, error) {
	if v.Type != RollupIntLong {
		return 0, errors.New("mismatched types")
	}
	return v.Value.(int64), nil
}

func (v PerfRollupValue) Float64() (float64, error) {
	if v.Type != RollupIntLong {
		return 0, errors.New("mismatched types")
	}
	return v.Value.(float64), nil
}

// TODO: implementations of these so that we can store just the
// int32/int64/float64 value in the database, but then have a better
// local experience for getting these values so its easier to safely
// get the values without accidentally truncating values.
func (v *PerfRollupValue) SetBSON(r legacyBSON.Raw) error                 { return nil }
func (v *PerfRollupValue) GetBSON() (interface{}, error)                  { return nil, nil }
func (v *PerfRollupValue) UnmarshalYAML(um func(interface{}) error) error { return nil }
func (v *PerfRollupValue) MarshalYAML() (interface{}, error)              { return nil, nil }
func (v *PerfRollupValue) MarshalJSON(in []byte) error                    { return nil }
func (v *PerfRollupValue) UnmarshalJSON(in []byte) error                  { return nil }
func (v *PerfRollupValue) MarshalBSON(in []byte) error                    { return nil }
func (v *PerfRollupValue) UnmarshalBSON(in []byte) error                  { return nil }
