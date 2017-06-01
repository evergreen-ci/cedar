package model

import (
	"github.com/evergreen-ci/sink/db"
	"github.com/evergreen-ci/sink/db/bsonutil"
	"gopkg.in/mgo.v2/bson"
)

// Metadata sub-documents are embedded in models to provide
// modification counters to ensure that updates don't overwrite
// interleaved changes to documents.
type Metadata struct {
	ModificationCount int            `bson:"nmod"`
	Version           int            `bson:"sver"`
	Units             map[string]int `bson:"units"`
}

var (
	metadataModificationCountKey = bsonutil.MustHaveTag(Metadata{}, "ModificationCount")
	metadataVersionKey           = bsonutil.MustHaveTag(Metadata{}, "Version")
	metadataUnitsKey             = bsonutil.MustHaveTag(Metadata{}, "Units")
)

func (m *Metadata) IsolatedUpdateQuery(metaDataKey string, id interface{}) *db.Q {
	modCount := bsonutil.GetDottedKeyName(metaDataKey, metadataModificationCountKey)

	query := db.Query(bson.M{
		"_id":    id,
		modCount: m.ModificationCount,
	})

	m.ModificationCount++

	return query
}
