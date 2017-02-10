package model

import "github.com/tychoish/sink/db/bsonutil"

type Metadata struct {
	Modifications int            `bson:"nmod"`
	Version       int            `bson:"version"`
	Units         map[string]int `bson:"units"`
}

var (
	VersionKey = bsonutil.MustHaveTag(Metadata{}, "Version")
	ModKey     = bsonutil.MustHaveTag(Metadata{}, "Modifications")
	UnitsKey   = bsonutil.MustHaveTag(Metadata{}, "Units")
)
