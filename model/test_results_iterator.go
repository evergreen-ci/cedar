package model

import (
	"context"
	"io/ioutil"
	"runtime"
	"sync"

	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"go.mongodb.org/mongo-driver/bson"
)

// TODO (EVG-14672): Remove this code and the relevant tests once the migration
// is complete.

const testResultsIteratorBatchSize = 100

// TestResultsIterator is an interface that enables iterating over test results.
type TestResultsIterator interface {
	Iterator
	// Item returns the current TestResult item held by the iterator.
	Item() TestResult
}

//////////////////
// Single Iterator
//////////////////

type testResultsIterator struct {
	bucket      pail.Bucket
	bucketItems []pail.BucketItem
	currentIdx  int
	items       chan TestResult
	currentItem TestResult
	exhausted   bool
	closed      bool
	catcher     grip.Catcher
}

// NewTestResultsIterator returns a TestResultsIterator.
func NewTestResultsIterator(bucket pail.Bucket) TestResultsIterator {
	return &testResultsIterator{
		bucket:  bucket,
		items:   make(chan TestResult, testResultsIteratorBatchSize),
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

	if len(i.bucketItems) == 0 {
		iter, err := i.bucket.List(ctx, "")
		if err != nil {
			i.catcher.Wrap(err, "listing bucket contents")
			return false
		}

		for iter.Next(ctx) {
			i.bucketItems = append(i.bucketItems, iter.Item())
		}
		if err = iter.Err(); err != nil {
			i.catcher.Wrapf(err, "iterating bucket contents")
			return false
		}
	}

	if len(i.items) == 0 {
		if i.currentIdx == len(i.bucketItems) {
			i.exhausted = true
			return false
		}

		if err := i.getNextBatch(ctx); err != nil {
			i.catcher.Add(err)
			return false
		}
	}

	select {
	case <-ctx.Done():
		i.catcher.Add(ctx.Err())
		return false
	case i.currentItem = <-i.items:
		return true
	}
}

func (i *testResultsIterator) getNextBatch(ctx context.Context) error {
	idxs := make(chan int, testResultsIteratorBatchSize)
	for j := 0; j < testResultsIteratorBatchSize; j++ {
		if i.currentIdx == len(i.bucketItems) {
			break
		}
		idxs <- i.currentIdx
		i.currentIdx++
	}
	close(idxs)
	var wg sync.WaitGroup
	catcher := grip.NewBasicCatcher()

	for j := 0; j < runtime.NumCPU(); j++ {
		wg.Add(1)
		go func() {
			defer func() {
				catcher.Add(recovery.HandlePanicWithError(recover(), nil, "test results iterator worker"))
				wg.Done()
			}()

			for idx := range idxs {
				if err := ctx.Err(); err != nil {
					catcher.Add(err)
					return
				}

				r, err := i.bucketItems[idx].Get(ctx)
				if err != nil {
					catcher.Wrapf(err, "getting test result '%s'", i.bucketItems[idx].Name())
					return
				}
				defer func() {
					grip.Error(message.WrapError(r.Close(), message.Fields{
						"message": "could not close bucket reader",
						"bucket":  i.bucket,
						"path":    i.bucketItems[idx].Name(),
					}))
				}()

				data, err := ioutil.ReadAll(r)
				if err != nil {
					catcher.Wrapf(err, "reading test result '%s'", i.bucketItems[idx].Name())
					return
				}

				var result TestResult
				if err := bson.Unmarshal(data, &result); err != nil {
					catcher.Wrapf(err, "unmarshalling test result '%s'", i.bucketItems[idx].Name())
					return
				}

				i.items <- result
			}
		}()
	}
	wg.Wait()

	return catcher.Resolve()
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

/////////////////
// Multi Iterator
/////////////////

type multiTestResultsIterator struct {
	its         []TestResultsIterator
	current     int
	currentItem TestResult
	exhausted   bool
	closed      bool
	catcher     grip.Catcher
}

// NewMultiTestResultsIterator returns a TestResultsIterator that iterates over
// over multiple TestResultsIterators.
func NewMultiTestResultsIterator(its ...TestResultsIterator) TestResultsIterator {
	return &multiTestResultsIterator{
		its:       its,
		exhausted: len(its) == 0,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *multiTestResultsIterator) Next(ctx context.Context) bool {
	if i.exhausted || i.closed {
		return false
	}

	defer func() {
		if i.catcher.HasErrors() {
			i.catcher.Wrap(i.Close(), "closing iterator")
		}
	}()

	for !i.its[i.current].Next(ctx) {
		i.catcher.Add(i.its[i.current].Err())
		i.current += 1
		if i.current == len(i.its) {
			i.exhausted = true
			return false
		}
	}

	i.currentItem = i.its[i.current].Item()
	return true
}

func (i *multiTestResultsIterator) Item() TestResult {
	return i.currentItem
}

func (i *multiTestResultsIterator) Exhausted() bool {
	return i.exhausted
}

func (i *multiTestResultsIterator) Err() error {
	return i.catcher.Resolve()
}

func (i *multiTestResultsIterator) Close() error {
	i.closed = true
	return nil
}
