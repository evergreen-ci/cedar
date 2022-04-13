package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const sysInfoCollection = "sysinfo.stats"

type SystemInformationRecord struct {
	ID        string             `bson:"_id" json:"id"`
	Timestamp time.Time          `bson:"ts" json:"time"`
	Data      message.SystemInfo `bson:"sysinfo" json:"sysinfo"`
	Hostname  string             `bson:"hn" json:"hostname"`
	populated bool
	env       cedar.Environment
}

var (
	sysInfoIDKey        = bsonutil.MustHaveTag(SystemInformationRecord{}, "ID")
	sysInfoTimestampKey = bsonutil.MustHaveTag(SystemInformationRecord{}, "Timestamp")
	sysInfoDataKey      = bsonutil.MustHaveTag(SystemInformationRecord{}, "Data")
	sysInfoHostKey      = bsonutil.MustHaveTag(SystemInformationRecord{}, "Hostname")
)

func (i *SystemInformationRecord) Setup(e cedar.Environment) { i.env = e }
func (i *SystemInformationRecord) IsNil() bool               { return !i.populated }

func (i *SystemInformationRecord) Save() error {
	if !i.populated {
		return errors.New("cannot save unpopulated record")
	}

	if i.ID == "" {
		i.ID = primitive.NewObjectID().String()
	}

	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(sysInfoCollection).Insert(i))
}

func (i *SystemInformationRecord) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	i.populated = false
	if err = session.DB(conf.DatabaseName).C(sysInfoCollection).FindId(i.ID).One(i); err != nil {
		return errors.Wrapf(err, "finding system information record '%s'", i.ID)
	}

	return nil
}

type SystemInformationRecords struct {
	slice     []*SystemInformationRecord
	populated bool
	env       cedar.Environment
}

func (i *SystemInformationRecords) Setup(e cedar.Environment)         { i.env = e }
func (i *SystemInformationRecords) IsNil() bool                       { return !i.populated }
func (i *SystemInformationRecords) Slice() []*SystemInformationRecord { return i.slice }
func (i *SystemInformationRecords) Size() int                         { return len(i.slice) }

func (i *SystemInformationRecords) runQuery(query db.Query) error {
	i.populated = false

	err := query.All(i.slice)
	if db.ResultsNotFound(err) {
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	i.populated = true

	return nil
}

func (i *SystemInformationRecords) FindHostname(host string, limit int) error {
	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	query := session.DB(conf.DatabaseName).C(sysInfoCollection).Find(map[string]interface{}{
		sysInfoHostKey: host,
	})

	if limit > 0 {
		query = query.Limit(limit)
	}

	return errors.WithStack(i.runQuery(query))
}

func (i *SystemInformationRecords) FindHostnameBetween(host string, before, after time.Time, limit int) error {
	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	query := session.DB(conf.DatabaseName).C(sysInfoCollection).Find(map[string]interface{}{
		sysInfoHostKey: host,
		sysInfoTimestampKey: bson.M{
			"$lt": before,
			"$gt": after,
		},
	})

	if limit > 0 {
		query = query.Limit(limit)
	}

	return errors.WithStack(i.runQuery(query))
}

func (i *SystemInformationRecords) FindBetween(before, after time.Time, limit int) error {
	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	query := session.DB(conf.DatabaseName).C(sysInfoCollection).Find(map[string]interface{}{
		sysInfoTimestampKey: map[string]interface{}{
			"$lt": before,
			"$gt": after,
		},
	})

	if limit > 0 {
		query = query.Limit(limit)
	}

	return errors.WithStack(i.runQuery(query))
}

func (i *SystemInformationRecords) CountBetween(before, after time.Time) (int, error) {
	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer session.Close()

	query := session.DB(conf.DatabaseName).C(sysInfoCollection).Find(map[string]interface{}{
		sysInfoTimestampKey: map[string]interface{}{
			"$lt": before,
			"$gt": after,
		},
	})

	c, err := query.Count()
	return c, errors.WithStack(err)
}

func (i *SystemInformationRecords) CountHostname(host string) (int, error) {
	conf, session, err := cedar.GetSessionWithConfig(i.env)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer session.Close()

	query := session.DB(conf.DatabaseName).C(sysInfoCollection).Find(map[string]interface{}{
		sysInfoHostKey: host,
	})

	c, err := query.Count()
	return c, errors.WithStack(err)
}
