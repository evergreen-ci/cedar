package model

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MessageEntry represents an entry for a "message" of a topic in the database.
type MessageEntry struct {
	// ForeignID is the id of a document located in another collection used by the
	// Topic interface to extract the actual message. The id of each message entry
	// is an int sequentially incremented for each message published to the topic.
	ForeignID string `bson:"foreign_id"`

	// ResumeToken and Error should be checked when watching a topic.
	ResumeToken []byte
	Err         error
}

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
	entry := &MessageEntry{ForeignID: foreignID}
	// TODO: should there be a collection per topic? Or, rather, one
	// collection for all topics, indexed by topic names.
	insertResult, err := env.GetDB().Collection(topic.Name()).InsertOne(ctx, entry)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   topic.Name(),
		"insertResult": insertResult,
		"op":           "publish to a topic",
	})

	return errors.Wrapf(err, "problem publishing to topic %s", topic.Name())
}

func watchTopic(ctx context.Context, env cedar.Environment, topic Topic, resumeToken []byte) (chan MessageEntry, error) {
	// TODO: Maybe need to set options FullDocument -> options.UpdateLookup.
	opts := options.ChangeStream()
	if len(resumeToken) > 0 {
		opts = opts.SetResumeAfter(resumeToken)
	}
	cs, err := env.GetDB().Collection(topic.Name()).Watch(ctx, mongo.Pipeline{}, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "problem getting change streams for topic %s", topic.Name())
	}

	// TODO: figure out appropriate buffer size.
	data := make(chan MessageEntry, 100)
	go func() {
		defer close(data)

		for cs.Next(ctx) {
			defer recovery.LogStackTraceAndContinue(fmt.Sprintf("watching topic %s", topic.Name()))

			resumeToken = []byte(cs.ResumeToken())
			entry := MessageEntry{}
			entry.ResumeToken = resumeToken

			if err = cs.Decode(&entry); err != nil {
				entry.Err = errors.Wrapf(err, "problem decoding message entry for topic %s", topic.Name())
			}

			select {
			case data <- entry:
				if entry.Err != nil {
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
			data <- MessageEntry{
				ResumeToken: resumeToken,
				Err:         errors.Wrapf(catcher.Resolve(), "cursor error for topic %s", topic.Name()),
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
