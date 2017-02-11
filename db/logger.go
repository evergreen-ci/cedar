package db

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
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
	Message     string      `bson:"message"`
	Payload     interface{} `bson:"pyaload"`
	MessageType string      `bson:"mtype"`
	Timestamp   time.Time   `bson:"ts"`
	Level       string      `bson:"level"`
}

var (
	eventMessageKey     = bsonutil.MustHaveTag(Event{}, "Message")
	eventPayloadKey     = bsonutil.MustHaveTag(Event{}, "Payload")
	eventMessageTypeKey = bsonutil.MustHaveTag(Event{}, "MessageType")
	eventTimestampKey   = bsonutil.MustHaveTag(Event{}, "Timestamp")
	eventLevelKey       = bsonutil.MustHaveTag(Event{}, "Level")
)

func NewEvent(m message.Composer) *Event {
	return &Event{
		Message:     m.String(),
		Payload:     m.Raw(),
		MessageType: fmt.Sprintf("%T", m),
		Timestamp:   time.Now(),
		Level:       m.Priority().String(),
	}
}

func (e *Event) Insert() error {
	return Insert(eventCollection, e)
}

type Events struct {
	slice     []Event
	populated bool
}

func (e *Events) Events() []Event { return e.slice }
func (e *Events) IsNil() bool     { return e.populated }

func (e *Events) FindLevel(level string, limit int) error {
	query := Query(bson.M{
		eventLevelKey: level,
	})

	e.populated = false
	err := FindAll(eventCollection, query, nil, []string{eventTimestampKey}, 0, limit, e.slice)
	if err != nil {
		return errors.WithStack(err)
	}
	e.populated = true

	return nil
}

// TODO add count method to events

////////////////////////////////////////
//
// Implementation of send.Sender

type mongoDBSender struct {
	*send.Base
}

func MakeDBSender() (send.Sender, error) {
	return NewDBSender("")
}

func NewDBSender(name string) (send.Sender, error) {
	s := &mongoDBSender{
		Base: send.NewBase(name),
	}

	err := s.SetErrorHandler(send.ErrorHandlerFromSender(grip.GetSender()))
	if err != nil {
		return nil, errors.Wrap(err, "problem getting default sender")
	}

	return s, nil
}

func (s *mongoDBSender) Send(m message.Composer) {
	if s.Level().ShouldLog(m) {
		e := NewEvent(m)
		if err := e.Insert(); err != nil {
			s.ErrorHandler(err, m)
		}
	}
}
