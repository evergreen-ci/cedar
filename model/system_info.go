package model

import (
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	"gopkg.in/mgo.v2/bson"
)

const sysInfoCollection = "sysinfo.stats"

type SystemInformationRecord struct {
	ID        bson.ObjectId      `bson:"_id" json:"id"`
	Timestamp time.Time          `bson:"ts" json:"time"`
	Data      message.SystemInfo `bson:"sysinfo" json:"sysinfo"`
	Hostname  string             `bson:"hn" json:"hostname"`
	populated bool
}

var (
	sysInfoIDKey        = bsonutil.MustHaveTag(SystemInformationRecord{}, "ID")
	sysInfoTimestampKey = bsonutil.MustHaveTag(SystemInformationRecord{}, "Timestamp")
	sysInfoDataKey      = bsonutil.MustHaveTag(SystemInformationRecord{}, "Data")
	sysInfoHostKey      = bsonutil.MustHaveTag(SystemInformationRecord{}, "Hostname")
)

func (i *SystemInformationRecord) Insert() error {
	if i.ID == "" {
		i.ID = bson.NewObjectId()
	}

	return errors.WithStack(db.Insert(sysInfoCollection, i))
}

func (i *SystemInformationRecord) FindID(id string) error {
	oid := bson.ObjectIdHex(id)

	query := db.Query(bson.M{
		sysInfoIDKey: oid,
	})

	i.populated = false
	if err := query.FindOne(sysInfoCollection, i); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type SystemInformationRecords struct {
	slice     []*SystemInformationRecord
	populated bool
}

func (i *SystemInformationRecords) IsNil() bool                       { return i.populated }
func (i *SystemInformationRecords) Slice() []*SystemInformationRecord { return i.slice }

func (i *SystemInformationRecords) runQuery(query *db.Q) error {
	i.populated = false
	if err := query.FindAll(sysInfoCollection, i.slice); err != nil {
		return errors.WithStack(err)
	}
	i.populated = true

	return nil
}

func (i *SystemInformationRecords) FindHostname(host string, limit int) error {
	query := db.Query(bson.M{
		sysInfoHostKey: host,
	})

	if limit > 0 {
		query.Limit(limit)
	}

	return errors.WithStack(i.runQuery(query))
}

func (i *SystemInformationRecords) FindHostnameBetween(host string, before, after time.Time, limit int) error {
	query := db.Query(bson.M{
		sysInfoHostKey: host,
		sysInfoTimestampKey: bson.M{
			"$lt": before,
			"$gt": after,
		},
	})

	if limit > 0 {
		query.Limit(limit)
	}

	return errors.WithStack(i.runQuery(query))
}

func (i *SystemInformationRecords) FindBetween(before, after time.Time, limit int) error {
	query := db.Query(bson.M{
		sysInfoTimestampKey: bson.M{
			"$lt": before,
			"$gt": after,
		},
	})

	if limit > 0 {
		query.Limit(limit)
	}

	return errors.WithStack(i.runQuery(query))
}

func (i *SystemInformationRecords) CountBetween(before, after time.Time) (int, error) {
	query := db.Query(bson.M{
		sysInfoTimestampKey: bson.M{
			"$lt": before,
			"$gt": after,
		},
	})

	c, err := query.Count(sysInfoCollection)
	err = errors.WithStack(err)

	return c, err
}

func (i *SystemInformationRecords) CountHostname(host string) (int, error) {
	query := db.Query(bson.M{
		sysInfoHostKey: host,
	})

	c, err := query.Count(sysInfoCollection)
	err = errors.WithStack(err)

	return c, err
}
