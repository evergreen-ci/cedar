package model

import (
	"context"
	"io/ioutil"

	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"go.mongodb.org/mongo-driver/bson"
)

// TestResultsIterator is an interface that enables iterating over test results.
type TestResultsIterator interface {
	Iterator
	// Item returns the current TestResult item held by the iterator.
	Item() TestResult
}

type testResultsIterator struct {
	bucket      pail.Bucket
	iter        pail.BucketIterator
	currentItem TestResult
	exhausted   bool
	closed      bool
	catcher     grip.Catcher
}

// NewTestResultsIterator returns an iterator
func NewTestResultsIterator(bucket pail.Bucket) TestResultsIterator {
	return &testResultsIterator{
		bucket:  bucket,
		catcher: grip.NewBasicCatcher(),
	}
}

func (i *testResultsIterator) Next(ctx context.Context) bool {
	if i.exhausted || i.closed {
		return false
	}

	defer func() {
		if i.catcher.HasErrors() {
			i.catcher.Wrap(i.Close(), "closing iterator")
		}
	}()

	if i.iter == nil {
		iter, err := i.bucket.List(ctx, "")
		if err != nil {
			i.catcher.Wrap(err, "listing bucket contents")
			return false
		}
		i.iter = iter
	}
	if !i.iter.Next(ctx) {
		i.catcher.Wrapf(i.iter.Err(), "iterating bucket contents")
		i.exhausted = true
		return false
	}

	r, err := i.iter.Item().Get(ctx)
	if err != nil {
		i.catcher.Wrapf(err, "item '%s'", i.iter.Item().Name())
		return false
	}
	defer func() {
		grip.Error(message.WrapError(r.Close(), message.Fields{
			"message": "could not close bucket reader",
			"bucket":  i.bucket,
			"path":    i.iter.Item().Name(),
		}))
	}()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		i.catcher.Wrapf(err, "reading test result '%s'", i.iter.Item().Name())
		return false
	}
	var result TestResult
	if err := bson.Unmarshal(data, &result); err != nil {
		i.catcher.Wrapf(err, "unmarshalling test result '%s'", i.iter.Item().Name())
		return false
	}

	i.currentItem = result

	return true
}

func (i *testResultsIterator) Item() TestResult {
	return i.currentItem
}

func (i *testResultsIterator) Exhausted() bool {
	return i.exhausted
}

func (i *testResultsIterator) Err() error {
	return i.catcher.Resolve()
}

func (i *testResultsIterator) Close() error {
	i.closed = true
	return nil
}
