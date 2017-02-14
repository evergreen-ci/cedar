package model

import "github.com/tychoish/sink/db/bsonutil"

// Metadata sub-documents are embedded in models to provide
// modification counters to ensure that updates don't overwrite
// interleaved changes to documents.
type Metadata struct {
	Modifications int            `bson:"nmod"`
	Version       int            `bson:"version"`
	Units         map[string]int `bson:"units"`
}

var (
	metadataModificationKey = bsonutil.MustHaveTag(Metadata{}, "Modifications")
	metadataVersionKey      = bsonutil.MustHaveTag(Metadata{}, "Version")
	metadataUnitsKey        = bsonutil.MustHaveTag(Metadata{}, "Units")
)
