package model

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TODO add index on { level: 1, Timestamp: 1 }
const eventCollection = "application.events"

///////////////////////////////////////////////////////////////////////////
//
// Models for Handling Events; in db to avoid circular dependency on model

// Event is a translation of
type Event struct {
	ID           string      `bson:"_id" json:"id"`
	Component    string      `bson:"com" json:"component"`
	Message      string      `bson:"m" json:"message"`
	Payload      interface{} `bson:"data" json:"payload"`
	MessageType  string      `bson:"mtype" json:"type"`
	Timestamp    time.Time   `bson:"ts" json:"time"`
	Level        string      `bson:"l" json:"level"`
	Acknowledged bool        `bson:"ack" json:"acknowledged"`

	populated bool
	env       cedar.Environment
}

var (
	eventIDKey           = bsonutil.MustHaveTag(Event{}, "ID")
	eventComponentKey    = bsonutil.MustHaveTag(Event{}, "Component")
	eventMessageKey      = bsonutil.MustHaveTag(Event{}, "Message")
	eventPayloadKey      = bsonutil.MustHaveTag(Event{}, "Payload")
	eventMessageTypeKey  = bsonutil.MustHaveTag(Event{}, "MessageType")
	eventTimestampKey    = bsonutil.MustHaveTag(Event{}, "Timestamp")
	eventLevelKey        = bsonutil.MustHaveTag(Event{}, "Level")
	eventAcknowledgedKey = bsonutil.MustHaveTag(Event{}, "Acknowledged")
)

func NewEvent(m message.Composer) *Event {
	return &Event{
		ID:          primitive.NewObjectID().String(),
		Message:     m.String(),
		Payload:     m.Raw(),
		MessageType: fmt.Sprintf("%T", m),
		Timestamp:   time.Now(),
		Level:       m.Priority().String(),
	}
}

func (e *Event) Setup(env cedar.Environment) { e.env = env }
func (e *Event) IsNil() bool                 { return !e.populated }
func (e *Event) Save() error {
	if e.ID == "" {
		return errors.New("cannot save an event without a populated ID")
	}

	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(e.sendLog(session.DB(conf.DatabaseName).C(eventCollection)))
}

func (e *Event) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	e.populated = false
	err = session.DB(conf.DatabaseName).C(eventCollection).FindId(e.ID).One(e)
	if err != nil {
		return errors.Wrapf(err, "finding event '%s'", e.ID)
	}

	e.populated = true

	return nil
}

func (e *Event) sendLog(coll db.Collection) error { return errors.WithStack(coll.Insert(e)) }

func (e *Event) Acknowledge() error {
	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	e.Acknowledged = true

	err = session.DB(conf.DatabaseName).C(eventCollection).UpdateId(e.ID, map[string]interface{}{
		eventAcknowledgedKey: map[string]interface{}{
			"$set": true,
		},
	})

	return errors.WithStack(err)
}

type Events struct {
	slice     []*Event
	populated bool
	env       cedar.Environment
}

func (e *Events) Setup(env cedar.Environment) { e.env = env }
func (e *Events) Slice() []*Event             { return e.slice }
func (e *Events) Size() int                   { return len(e.slice) }
func (e *Events) IsNil() bool                 { return !e.populated }

func (e *Events) FindLevel(level string, limit int) error {
	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	query := e.levelQuery(conf, session, level)
	if limit > 0 {
		query = query.Limit(limit)
	}

	e.populated = false
	err = query.All(e.slice)
	if db.ResultsNotFound(err) {
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	e.populated = true

	return nil
}

func (e *Events) levelQuery(conf *cedar.Configuration, session db.Session, level string) db.Query {
	coll := session.DB(conf.DatabaseName).C(eventCollection)
	return coll.Find(map[string]interface{}{eventLevelKey: level})
}

func (e *Events) CountLevel(level string) (int, error) {
	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer session.Close()
	return e.levelQuery(conf, session, level).Count()
}

func (e *Events) Count() (int, error) {
	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer session.Close()

	return session.DB(conf.DatabaseName).C(eventCollection).Count()
}

////////////////////////////////////////
//
// Implementation of send.Sender

type mongoDBSender struct {
	*send.Base
	env        cedar.Environment
	session    db.Session
	collection db.Collection
}

func MakeDBSender(e cedar.Environment) (send.Sender, error) { return NewDBSender(e, "") }

func NewDBSender(e cedar.Environment, name string) (send.Sender, error) {
	s := &mongoDBSender{
		env:  e,
		Base: send.NewBase(name),
	}
	conf, session, err := cedar.GetSessionWithConfig(s.env)
	if err != nil {
		return nil, errors.Wrap(err, "getting DB session and config")
	}
	s.session = session
	s.collection = s.session.DB(conf.DatabaseName).C(eventCollection)

	err = s.SetErrorHandler(send.ErrorHandlerFromSender(grip.GetSender()))
	if err != nil {
		return nil, errors.Wrap(err, "getting default sender")
	}

	return s, nil
}

func (s *mongoDBSender) Send(m message.Composer) {
	if s.Level().ShouldLog(m) {
		e := NewEvent(m)
		e.Component = s.Name()

		if err := e.sendLog(s.collection); err != nil {
			s.ErrorHandler()(err, m)
		}
	}
}

func (s *mongoDBSender) Flush(_ context.Context) error { return nil }

func (s *mongoDBSender) Close() error { s.session.Close(); return nil }
