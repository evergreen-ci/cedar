package model

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
)

// LogIterator is an interface that enables iterating over lines of buildlogger
// logs.
type LogIterator interface {
	// Next returns true if the iterator has not yet been exhausted, false
	// otherwise.
	Next(context.Context) bool
	// Err returns any errors that are captured by the iterator.
	Err() error
	// Item returns the current LogLine item held by the iterator.
	Item() LogLine
	// Close closes the iterator, this function should be called once the
	// iterator is no longer needed.
	Close() error
}

//////////////////////
// Serialized Iterator
//////////////////////
type serializedIterator struct {
	bucket            pail.Bucket
	chunks            []LogChunkInfo
	timeRange         util.TimeRange
	lineCount         int
	keyIndex          int
	currentReadCloser io.ReadCloser
	currentReader     *bufio.Reader
	currentItem       LogLine
	catcher           grip.Catcher
}

// NewSerializedLogIterator returns a LogIterator that serially fetches
// chunks from blob storage while iterating over lines of a buildlogger log.
func NewSerializedLogIterator(bucket pail.Bucket, chunks []LogChunkInfo, timeRange util.TimeRange) LogIterator {
	chunks = filterChunks(timeRange, chunks)

	return &serializedIterator{
		bucket:    bucket,
		chunks:    chunks,
		timeRange: timeRange,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *serializedIterator) Next(ctx context.Context) bool {
	for {
		if i.currentReader == nil {
			if i.keyIndex >= len(i.chunks) {
				return false
			}

			var err error
			i.currentReadCloser, err = i.bucket.Get(ctx, i.chunks[i.keyIndex].Key)
			if err != nil {
				i.catcher.Add(errors.Wrap(err, "problem downloading log artifact"))
				return false
			}
			i.currentReader = bufio.NewReader(i.currentReadCloser)
		}

		data, err := i.currentReader.ReadString('\n')
		if err == io.EOF {
			if i.lineCount != i.chunks[i.keyIndex].NumLines {
				i.catcher.Add(errors.New("corrupt data"))
			}

			i.catcher.Add(errors.Wrap(i.currentReadCloser.Close(), "problem closing ReadCloser"))
			i.currentReadCloser = nil
			i.currentReader = nil
			i.lineCount = 0
			i.keyIndex++

			return i.Next(ctx)
		}
		if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem getting line"))
			return false
		}

		i.currentItem, err = parseLogLineString(data)
		if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem parsing timestamp"))
			return false
		}
		i.lineCount++

		if i.currentItem.Timestamp.After(i.timeRange.EndAt) {
			return false
		}
		if i.currentItem.Timestamp.After(i.timeRange.StartAt) ||
			i.currentItem.Timestamp.Equal(i.timeRange.StartAt) {
			break
		}
	}

	return true
}

func (i *serializedIterator) Err() error { return i.catcher.Resolve() }

func (i *serializedIterator) Item() LogLine { return i.currentItem }

func (i *serializedIterator) Close() error {
	if i.currentReadCloser != nil {
		return i.currentReadCloser.Close()
	}

	return nil
}

///////////////////
// Batched Iterator
///////////////////
type batchedIterator struct {
	bucket        pail.Bucket
	batchSize     int
	chunks        []LogChunkInfo
	chunkIndex    int
	timeRange     util.TimeRange
	lineCount     int
	keyIndex      int
	readers       map[string]io.ReadCloser
	currentReader *bufio.Reader
	currentItem   LogLine
	catcher       grip.Catcher
}

// NewBatchedLog returns a LogIterator that fetches batches (size set by the
// caller) of chunks from blob storage in parallel while iterating over lines
// of a buildlogger log.
func NewBatchedLogIterator(bucket pail.Bucket, chunks []LogChunkInfo, batchSize int, timeRange util.TimeRange) LogIterator {
	chunks = filterChunks(timeRange, chunks)

	return &batchedIterator{
		bucket:    bucket,
		batchSize: batchSize,
		chunks:    chunks,
		timeRange: timeRange,
		catcher:   grip.NewBasicCatcher(),
	}
}

// NewParallelizedLogIterator returns a LogIterator that fetches all chunks
// from blob storage in parallel while iterating over lines of a buildlogger
// log.
func NewParallelizedLogIterator(bucket pail.Bucket, chunks []LogChunkInfo, timeRange util.TimeRange) LogIterator {
	chunks = filterChunks(timeRange, chunks)

	return &batchedIterator{
		bucket:    bucket,
		batchSize: len(chunks),
		chunks:    chunks,
		timeRange: timeRange,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *batchedIterator) getNextBatch(ctx context.Context) error {
	if err := i.Close(); err != nil {
		return errors.Wrap(err, "problem closing readers")
	}

	end := i.chunkIndex + i.batchSize
	if end > len(i.chunks) {
		end = len(i.chunks)
	}
	work := make(chan LogChunkInfo, end-i.chunkIndex)
	for _, chunk := range i.chunks[i.chunkIndex:end] {
		work <- chunk
	}
	close(work)
	var wg sync.WaitGroup
	var mux sync.Mutex
	readers := map[string]io.ReadCloser{}
	catcher := grip.NewBasicCatcher()

	for j := 0; j < runtime.NumCPU(); j++ {
		wg.Add(1)
		go func() {
			defer func() {
				var err error
				catcher.Add(recovery.HandlePanicWithError(recover(), err))
				wg.Done()
			}()

			for chunk := range work {
				if err := ctx.Err(); err != nil {
					catcher.Add(err)
					return
				} else {
					r, err := i.bucket.Get(ctx, chunk.Key)
					if err != nil {
						catcher.Add(err)
						return
					}
					mux.Lock()
					readers[chunk.Key] = r
					mux.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	i.chunkIndex = end
	i.readers = readers
	return errors.Wrap(catcher.Resolve(), "problem downloading log artifacts")
}

func (i *batchedIterator) Next(ctx context.Context) bool {
	for {
		if i.currentReader == nil {
			if i.keyIndex >= len(i.chunks) {
				return false
			}

			reader, ok := i.readers[i.chunks[i.keyIndex].Key]
			if !ok {
				if err := i.getNextBatch(ctx); err != nil {
					i.catcher.Add(err)
					return false
				}
				continue
			}

			i.currentReader = bufio.NewReader(reader)
		}

		data, err := i.currentReader.ReadString('\n')
		if err == io.EOF {
			if i.lineCount != i.chunks[i.keyIndex].NumLines {
				i.catcher.Add(errors.New("corrupt data"))
			}

			i.currentReader = nil
			i.lineCount = 0
			i.keyIndex++

			return i.Next(ctx)
		} else if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem getting line"))
			return false
		}

		i.currentItem, err = parseLogLineString(data)
		if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem parsing timestamp"))
			return false
		}
		i.lineCount++

		if i.currentItem.Timestamp.After(i.timeRange.EndAt) {
			return false
		}
		if i.currentItem.Timestamp.After(i.timeRange.StartAt) ||
			i.currentItem.Timestamp.Equal(i.timeRange.StartAt) {
			break
		}
	}

	return true
}

func (i *batchedIterator) Err() error { return i.catcher.Resolve() }

func (i *batchedIterator) Item() LogLine { return i.currentItem }

func (i *batchedIterator) Close() error {
	catcher := grip.NewBasicCatcher()

	for _, r := range i.readers {
		catcher.Add(r.Close())
	}

	return catcher.Resolve()
}

///////////////////
// Helper functions
///////////////////

func parseLogLineString(data string) (LogLine, error) {
	ts, err := strconv.ParseInt(strings.TrimSpace(data[:20]), 10, 64)

	if err != nil {
		return LogLine{}, err
	}

	return LogLine{
		Timestamp: time.Unix(0, ts*1e6).UTC(),
		Data:      data[20:],
	}, nil
}

func prependTimestamp(t time.Time, data string) string {
	ts := fmt.Sprintf("%20d", util.UnixMilli(t))

	return fmt.Sprintf("%s%s\n", ts, data)
}

func filterChunks(timeRange util.TimeRange, chunks []LogChunkInfo) []LogChunkInfo {
	filteredChunks := []LogChunkInfo{}
	for _, chunk := range chunks {
		if timeRange.Check(chunk.Start) || timeRange.Check(chunk.End) {
			filteredChunks = append(filteredChunks, chunk)
		}
	}

	return filteredChunks
}
