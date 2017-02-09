package model

import "github.com/tychoish/sink/db/bsonutil"

type Metadata struct {
	Modifications int `bson:"nmod"`
	Version       int `bson:"version"`
}

var VersionKey = bsonutil.MustHaveTag(Metadata{}, "Version")
var ModKey = bsonutil.MustHaveTag(Metadata{}, "Modifications")
