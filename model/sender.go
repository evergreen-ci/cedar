package model

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	"gopkg.in/mgo.v2/bson"
)

// TODO add index on { level: 1, Timestamp: 1 }
const eventCollection = "application.events"

///////////////////////////////////////////////////////////////////////////
//
// Models for Handling Events; in db to avoid circular dependency on model

// Event is a translation of
type Event struct {
	ID           bson.ObjectId `bson:"_id" json:"id"`
	Component    string        `bson:"com" json:"component"`
	Message      string        `bson:"m" json:"message"`
	Payload      interface{}   `bson:"data" json:"payload"`
	MessageType  string        `bson:"mtype" json:"type"`
	Timestamp    time.Time     `bson:"ts" json:"time"`
	Level        string        `bson:"l" json:"level"`
	Acknowledged bool          `bson:"ack" json:"acknowledged"`
}

var (
	eventComponentKey   = bsonutil.MustHaveTag(Event{}, "Component")
	eventMessageKey     = bsonutil.MustHaveTag(Event{}, "Message")
	eventPayloadKey     = bsonutil.MustHaveTag(Event{}, "Payload")
	eventMessageTypeKey = bsonutil.MustHaveTag(Event{}, "MessageType")
	eventTimestampKey   = bsonutil.MustHaveTag(Event{}, "Timestamp")
	eventLevelKey       = bsonutil.MustHaveTag(Event{}, "Level")
)

func NewEvent(m message.Composer) *Event {
	return &Event{
		ID:          bson.NewObjectId(),
		Message:     m.String(),
		Payload:     m.Raw(),
		MessageType: fmt.Sprintf("%T", m),
		Timestamp:   time.Now(),
		Level:       m.Priority().String(),
	}
}

func (e *Event) IsNil() bool   { return e.populated }
func (e *Event) Insert() error { return db.Insert(eventCollection, e) }

func (e *Event) FindID(id string) error {
	oid := bson.ObjectIdHex(id)
	e.ID = oid

	e.populated = false
	if err := db.FindOne(eventCollection, e, interface{}, []string{}, e); err != nil {
		return errors.WithStack(err)
	}
	e.populated = false

	return nil
}

func (e *Event) Acknowledge() error {
	e.Acknowledged = true
	return db.UpdateID(e.ID, e)
}

type Events struct {
	slice     []Event
	populated bool
}

func (e *Events) Events() []Event { return e.slice }
func (e *Events) IsNil() bool     { return e.populated }

func (e *Events) FindLevel(level string, limit int) error {
	query := db.Query(bson.M{
		eventLevelKey: level,
	})

	e.populated = false
	err := db.FindAll(eventCollection, query, nil, []string{eventTimestampKey}, 0, limit, e.slice)
	if err != nil {
		return errors.WithStack(err)
	}
	e.populated = true

	return nil
}

func (e *Events) CountLevel(level string) (int, error) {
	return db.Query(bson.M{eventLevelKey: level}).Count()
}

func (e *Events) Count() (int, error) {
	return db.Query(bson.M{}).Count()
}

// TODO add count method to events

////////////////////////////////////////
//
// Implementation of send.Sender

type mongoDBSender struct{ *send.Base }

func MakeDBSender() (send.Sender, error) { return NewDBSender("") }

func NewDBSender(name string) (send.Sender, error) {
	s := &mongoDBSender{send.NewBase(name)}

	err := s.SetErrorHandler(send.ErrorHandlerFromSender(grip.GetSender()))
	if err != nil {
		return nil, errors.Wrap(err, "problem getting default sender")
	}

	return s, nil
}

func (s *mongoDBSender) Send(m message.Composer) {
	if s.Level().ShouldLog(m) {
		e := NewEvent(m)
		e.Component = s.Name()
		if err := e.Insert(); err != nil {
			s.ErrorHandler(err, m)
		}
	}
}
