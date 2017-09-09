package model

import (
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/bsonutil"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/anser/db"
	"gopkg.in/mgo.v2/bson"
)

const sysInfoCollection = "sysinfo.stats"

type SystemInformationRecord struct {
	ID        string             `bson:"_id" json:"id"`
	Timestamp time.Time          `bson:"ts" json:"time"`
	Data      message.SystemInfo `bson:"sysinfo" json:"sysinfo"`
	Hostname  string             `bson:"hn" json:"hostname"`
	populated bool
	env       sink.Environment
}

var (
	sysInfoIDKey        = bsonutil.MustHaveTag(SystemInformationRecord{}, "ID")
	sysInfoTimestampKey = bsonutil.MustHaveTag(SystemInformationRecord{}, "Timestamp")
	sysInfoDataKey      = bsonutil.MustHaveTag(SystemInformationRecord{}, "Data")
	sysInfoHostKey      = bsonutil.MustHaveTag(SystemInformationRecord{}, "Hostname")
)

func (i *SystemInformationRecord) Setup(e sink.Environment) { i.env = e }
func (i *SystemInformationRecord) IsNil() bool              { return i.populated }

func (i *SystemInformationRecord) Insert() error {
	if i.ID == "" {
		i.ID = string(bson.NewObjectId())
	}

	conf, session, err := sink.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(sysInfoCollection).Insert(i))
}

func (i *SystemInformationRecord) FindID(id string) error {
	conf, session, err := sink.GetSessionWithConfig(i.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	i.populated = false
	err = session.DB(conf.DatabaseName).C(sysInfoCollection).FindId(id).One(i)
	if db.ResultsNotFound(err) {
		return nil
	}

	return errors.WithStack(err)
}

type SystemInformationRecords struct {
	slice     []*SystemInformationRecord
	populated bool
	env       sink.Environment
}

func (i *SystemInformationRecords) Setup(e sink.Environment)          { i.env = e }
func (i *SystemInformationRecords) IsNil() bool                       { return i.populated }
func (i *SystemInformationRecords) Slice() []*SystemInformationRecord { return i.slice }

func (i *SystemInformationRecords) runQuery(query db.Query) error {
	i.populated = false
	err := query.All(i.slice)
	if db.ResultsNotFound(err) {
		return nil
	}

	if err != nil {
		return errors.WithStack(err)
	}

	i.populated = true

	return nil
}

func (i *SystemInformationRecords) FindHostname(host string, limit int) error {
	conf, session, err := sink.GetSessionWithConfig(i.env)
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
	conf, session, err := sink.GetSessionWithConfig(i.env)
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
	conf, session, err := sink.GetSessionWithConfig(i.env)
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
	conf, session, err := sink.GetSessionWithConfig(i.env)
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
	conf, session, err := sink.GetSessionWithConfig(i.env)
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
