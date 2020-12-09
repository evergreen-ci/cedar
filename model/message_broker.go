package model

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const topicsCollection = "topics"

// MessageEntry represents an entry for a "message" of a topic in the database.
type MessageEntry struct {
	// Topic is the name of the topic.
	Topic string `bson:"topic"`
	// ForeignID is the id of a document located in another collection used
	// by the Topic interface to extract the actual message.
	ForeignID string `bson:"foreign_id"`

	// ResumeToken and Error should be checked when watching a topic.
	ResumeToken []byte `bson:"-"`
	Err         error  `bson:"-"`
}

var (
	messageEntryTopicKey     = bsonutil.MustHaveTag(MessageEntry{}, "Topic")
	messageEntryForeignIDKey = bsonutil.MustHaveTag(MessageEntry{}, "ForeignID")
)

// PublishToTopic creates a new message entry with the given id in the
// corresponding collection for the given topic. If the topic does not exist in
// the global registry, an error is returned.
func PublishToTopic(ctx context.Context, env cedar.Environment, name string, id string) error {
	topic, err := GetTopic(name)
	if err != nil {
		return err
	}

	return publishToTopic(ctx, env, topic, id)
}

// WatchTopic returns a MessageEntry channel fed by a change stream on the
// topic's collection. The resume token specifies where to start after with the
// change stream. If the topic does not exist in the global registry, an error
// is returned.
func WatchTopic(ctx context.Context, env cedar.Environment, name string, resumeToken []byte) (chan MessageEntry, error) {
	topic, err := GetTopic(name)
	if err != nil {
		return nil, err
	}

	return watchTopic(ctx, env, topic, resumeToken)
}

// publishToTopic creates a new message entry with the foreign ID in the
// corresponding collection for the given topic.
func publishToTopic(ctx context.Context, env cedar.Environment, topic Topic, foreignID string) error {
	entry := &MessageEntry{
		Topic:     topic.Name(),
		ForeignID: foreignID,
	}
	insertResult, err := env.GetDB().Collection(topicsCollection).InsertOne(ctx, entry)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   topicsCollection,
		"insertResult": insertResult,
		"op":           fmt.Sprintf("publish to topic %s", topic.Name()),
	})

	return errors.Wrapf(err, "problem publishing to topic %s", topic.Name())
}

func watchTopic(ctx context.Context, env cedar.Environment, topic Topic, resumeToken []byte) (chan MessageEntry, error) {
	opts := options.ChangeStream()
	if len(resumeToken) > 0 {
		opts = opts.SetStartAfter(resumeToken)
	}
	cs, err := env.GetDB().Collection(topicsCollection).Watch(
		ctx,
		mongo.Pipeline{{{
			Key: "$match",
			Value: bson.D{{
				Key:   bsonutil.GetDottedKeyName("fullDocument", messageEntryTopicKey),
				Value: topic.Name(),
			}},
		}}},
		opts,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "problem getting change streams for topic %s", topic.Name())
	}

	// TODO: figure out appropriate buffer size.
	data := make(chan MessageEntry, 100)
	go func() {
		defer recovery.LogStackTraceAndContinue(fmt.Sprintf("watching topic %s", topic.Name()))
		defer close(data)

		for cs.Next(ctx) {
			event := struct {
				Entry MessageEntry `bson:"fullDocument"`
			}{}
			resumeToken = []byte(cs.ResumeToken())
			event.Entry.ResumeToken = resumeToken

			if err = cs.Decode(&event); err != nil {
				event.Entry.Err = errors.Wrapf(err, "problem decoding message entry for topic %s", topic.Name())
			}

			select {
			case data <- event.Entry:
				if event.Entry.Err != nil {
					break
				}
			case <-ctx.Done():
				break
			}
		}

		catcher := grip.NewBasicCatcher()
		catcher.Add(cs.Err())
		catcher.Add(cs.Close(context.Background()))
		if catcher.HasErrors() {
			select {
			case data <- MessageEntry{
				ResumeToken: resumeToken,
				Err:         errors.Wrapf(catcher.Resolve(), "cursor error for topic %s", topic.Name()),
			}:
			case <-ctx.Done():
			}
		}
	}()

	return data, nil
}

// Topic is an interface representing a topic for publishing and subscribing
// via the message broker.
type Topic interface {
	// Name returns the unique name of the topic.
	Name() string
	// ExtractMessage searches the database and returns a consistently
	// structured message extracted from the document with the given id.
	ExtractMessage(id string) ([]byte, error)
}

// topicRegistry is the source of truth for all topics, this is created upon
// startup and remains unchanged.
var topicRegistry = map[string]Topic{}

// GetTopic returns the topic with the given name from the topic registry, if
// it does not exist an error is returned.
func GetTopic(name string) (Topic, error) {
	topic, ok := topicRegistry[name]
	if !ok {
		return nil, errors.Errorf("topic with name %s does not exist", name)
	}

	return topic, nil
}

// TODO: add topics to registry.
