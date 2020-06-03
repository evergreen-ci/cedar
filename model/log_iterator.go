package model

import (
	"bufio"
	"container/heap"
	"context"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
)

// Iterator represents a cursor for generic iteration over a sequence of items.
type Iterator interface {
	// Next returns true if the iterator has not yet been exhausted or
	// closed, false otherwise.
	Next(context.Context) bool
	// Exhausted returns true if the iterator has not yet been exhausted,
	// regardless if it has been closed or not.
	Exhausted() bool
	// Err returns any errors that are captured by the iterator.
	Err() error
	// Close closes the iterator. This function should be called once the
	// iterator is no longer needed.
	Close() error
}

// LogIterator is an interface that enables iterating over lines of buildlogger
// logs.
// kim: NOTE: this can be used for TestResults iterator.
type LogIterator interface {
	Iterator
	// Item returns the current LogLine item held by the iterator.
	Item() LogLine
	// Reverse returns a reversed copy of the iterator.
	Reverse() LogIterator
	// IsReversed returns true if the iterator is in reverse order and
	// false otherwise.
	IsReversed() bool
}

//////////////////////
// Serialized Iterator
//////////////////////
type serializedIterator struct {
	bucket               pail.Bucket
	chunks               []LogChunkInfo
	timeRange            TimeRange
	reverse              bool
	lineCount            int
	keyIndex             int
	currentReadCloser    io.ReadCloser
	currentReverseReader *reverseLineReader
	currentReader        *bufio.Reader
	currentItem          LogLine
	catcher              grip.Catcher
	exhausted            bool
	closed               bool
}

// NewSerializedLogIterator returns a LogIterator that serially fetches
// chunks from blob storage while iterating over lines of a buildlogger log.
func NewSerializedLogIterator(bucket pail.Bucket, chunks []LogChunkInfo, timeRange TimeRange) LogIterator {
	chunks = filterChunks(timeRange, chunks)

	return &serializedIterator{
		bucket:    bucket,
		chunks:    chunks,
		timeRange: timeRange,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *serializedIterator) Reverse() LogIterator {
	chunks := make([]LogChunkInfo, len(i.chunks))
	_ = copy(chunks, i.chunks)
	reverseChunks(chunks)

	return &serializedIterator{
		bucket:    i.bucket,
		chunks:    chunks,
		timeRange: i.timeRange,
		reverse:   !i.reverse,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *serializedIterator) IsReversed() bool { return i.reverse }

func (i *serializedIterator) Next(ctx context.Context) bool {
	if i.closed {
		return false
	}

	for {
		if i.currentReader == nil && i.currentReverseReader == nil {
			if i.keyIndex >= len(i.chunks) {
				i.exhausted = true
				return false
			}

			var err error
			i.currentReadCloser, err = i.bucket.Get(ctx, i.chunks[i.keyIndex].Key)
			if err != nil {
				i.catcher.Add(errors.Wrap(err, "problem downloading log artifact"))
				return false
			}
			if i.reverse {
				i.currentReverseReader = newReverseLineReader(i.currentReadCloser)
			} else {
				i.currentReader = bufio.NewReader(i.currentReadCloser)
			}
		}

		var data string
		var err error
		if i.reverse {
			data, err = i.currentReverseReader.ReadLine()
		} else {
			data, err = i.currentReader.ReadString('\n')
		}
		if err == io.EOF {
			if i.lineCount != i.chunks[i.keyIndex].NumLines {
				i.catcher.Add(errors.New("corrupt data"))
			}

			i.catcher.Add(errors.Wrap(i.currentReadCloser.Close(), "problem closing ReadCloser"))
			i.currentReadCloser = nil
			i.currentReverseReader = nil
			i.currentReader = nil
			i.lineCount = 0
			i.keyIndex++

			return i.Next(ctx)
		}
		if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem getting line"))
			return false
		}

		item, err := parseLogLineString(data)
		if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem parsing timestamp"))
			return false
		}
		i.lineCount++

		if item.Timestamp.After(i.timeRange.EndAt) && !i.reverse {
			i.exhausted = true
			return false
		}
		if item.Timestamp.Before(i.timeRange.StartAt) && i.reverse {
			i.exhausted = true
			return false
		}

		if (item.Timestamp.After(i.timeRange.StartAt) || item.Timestamp.Equal(i.timeRange.StartAt)) &&
			(item.Timestamp.Before(i.timeRange.EndAt) || item.Timestamp.Equal(i.timeRange.EndAt)) {
			i.currentItem = item
			break
		}
	}

	return true
}

func (i *serializedIterator) Exhausted() bool { return i.exhausted }

func (i *serializedIterator) Err() error { return i.catcher.Resolve() }

func (i *serializedIterator) Item() LogLine { return i.currentItem }

func (i *serializedIterator) Close() error {
	i.closed = true
	if i.currentReadCloser != nil {
		return i.currentReadCloser.Close()
	}

	return nil
}

///////////////////
// Batched Iterator
///////////////////
type batchedIterator struct {
	bucket               pail.Bucket
	batchSize            int
	chunks               []LogChunkInfo
	chunkIndex           int
	timeRange            TimeRange
	reverse              bool
	lineCount            int
	keyIndex             int
	readers              map[string]io.ReadCloser
	currentReverseReader *reverseLineReader
	currentReader        *bufio.Reader
	currentItem          LogLine
	catcher              grip.Catcher
	exhausted            bool
	closed               bool
}

// NewBatchedLog returns a LogIterator that fetches batches (size set by the
// caller) of chunks from blob storage in parallel while iterating over lines
// of a buildlogger log.
func NewBatchedLogIterator(bucket pail.Bucket, chunks []LogChunkInfo, batchSize int, timeRange TimeRange) LogIterator {
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
func NewParallelizedLogIterator(bucket pail.Bucket, chunks []LogChunkInfo, timeRange TimeRange) LogIterator {
	chunks = filterChunks(timeRange, chunks)

	return &batchedIterator{
		bucket:    bucket,
		batchSize: len(chunks),
		chunks:    chunks,
		timeRange: timeRange,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *batchedIterator) Reverse() LogIterator {
	chunks := make([]LogChunkInfo, len(i.chunks))
	_ = copy(chunks, i.chunks)
	reverseChunks(chunks)

	return &batchedIterator{
		bucket:    i.bucket,
		batchSize: i.batchSize,
		chunks:    chunks,
		timeRange: i.timeRange,
		reverse:   !i.reverse,
		catcher:   grip.NewBasicCatcher(),
	}
}

func (i *batchedIterator) IsReversed() bool { return i.reverse }

func (i *batchedIterator) getNextBatch(ctx context.Context) error {
	catcher := grip.NewBasicCatcher()
	for _, r := range i.readers {
		catcher.Add(r.Close())
	}
	if err := catcher.Resolve(); err != nil {
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
	catcher = grip.NewBasicCatcher()

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
	if i.closed {
		return false
	}

	for {
		if i.currentReader == nil && i.currentReverseReader == nil {
			if i.keyIndex >= len(i.chunks) {
				i.exhausted = true
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

			if i.reverse {
				i.currentReverseReader = newReverseLineReader(reader)
			} else {
				i.currentReader = bufio.NewReader(reader)
			}
		}

		var data string
		var err error
		if i.reverse {
			data, err = i.currentReverseReader.ReadLine()
		} else {
			data, err = i.currentReader.ReadString('\n')
		}
		if err == io.EOF {
			if i.lineCount != i.chunks[i.keyIndex].NumLines {
				i.catcher.Add(errors.New("corrupt data"))
			}

			i.currentReverseReader = nil
			i.currentReader = nil
			i.lineCount = 0
			i.keyIndex++

			return i.Next(ctx)
		} else if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem getting line"))
			return false
		}

		item, err := parseLogLineString(data)
		if err != nil {
			i.catcher.Add(errors.Wrap(err, "problem parsing timestamp"))
			return false
		}
		i.lineCount++

		if item.Timestamp.After(i.timeRange.EndAt) && !i.reverse {
			i.exhausted = true
			return false
		}
		if item.Timestamp.Before(i.timeRange.StartAt) && i.reverse {
			i.exhausted = true
			return false
		}
		if (item.Timestamp.After(i.timeRange.StartAt) || item.Timestamp.Equal(i.timeRange.StartAt)) &&
			(item.Timestamp.Before(i.timeRange.EndAt) || item.Timestamp.Equal(i.timeRange.EndAt)) {
			i.currentItem = item
			break
		}
	}

	return true
}

func (i *batchedIterator) Exhausted() bool { return i.exhausted }

func (i *batchedIterator) Err() error { return i.catcher.Resolve() }

func (i *batchedIterator) Item() LogLine { return i.currentItem }

func (i *batchedIterator) Close() error {
	i.closed = true
	catcher := grip.NewBasicCatcher()

	for _, r := range i.readers {
		catcher.Add(r.Close())
	}

	return catcher.Resolve()
}

///////////////////
// Merging Iterator
///////////////////

type mergingIterator struct {
	iterators    []LogIterator
	iteratorHeap *LogIteratorHeap
	currentItem  LogLine
	catcher      grip.Catcher
	started      bool
}

// NewMergeIterator returns a LogIterator that merges N buildlogger logs,
// passed in as LogIterators, respecting the order of each line's timestamp.
func NewMergingIterator(iterators ...LogIterator) LogIterator {
	return &mergingIterator{
		iterators:    iterators,
		iteratorHeap: &LogIteratorHeap{min: true},
		catcher:      grip.NewBasicCatcher(),
	}
}

func (i *mergingIterator) Reverse() LogIterator {
	for j := range i.iterators {
		if !i.iterators[j].IsReversed() {
			i.iterators[j] = i.iterators[j].Reverse()
		}
	}

	return &mergingIterator{
		iterators:    i.iterators,
		iteratorHeap: &LogIteratorHeap{min: false},
		catcher:      grip.NewBasicCatcher(),
	}
}

func (i *mergingIterator) IsReversed() bool { return !i.iteratorHeap.min }

func (i *mergingIterator) Next(ctx context.Context) bool {
	if !i.started {
		i.init(ctx)
	}

	it := i.iteratorHeap.SafePop()
	if it == nil {
		return false
	}
	i.currentItem = it.Item()

	if it.Next(ctx) {
		i.iteratorHeap.SafePush(it)
	} else {
		i.catcher.Add(it.Err())
		i.catcher.Add(it.Close())
		if i.catcher.HasErrors() {
			return false
		}
	}

	return true
}

func (i *mergingIterator) Exhausted() bool {
	for _, it := range i.iterators {
		if it.Exhausted() {
			return true
		}
	}

	return false
}

func (i *mergingIterator) init(ctx context.Context) {
	heap.Init(i.iteratorHeap)

	for j := range i.iterators {
		if i.iterators[j].Next(ctx) {
			i.iteratorHeap.SafePush(i.iterators[j])
		}

		// fail early
		if i.iterators[j].Err() != nil {
			i.catcher.Add(i.iterators[j].Err())
			i.iteratorHeap = &LogIteratorHeap{}
			break
		}
	}

	i.started = true
}

func (i *mergingIterator) Err() error { return i.catcher.Resolve() }

func (i *mergingIterator) Item() LogLine { return i.currentItem }

func (i *mergingIterator) Close() error {
	catcher := grip.NewBasicCatcher()

	for {
		it := i.iteratorHeap.SafePop()
		if it == nil {
			break
		}
		catcher.Add(it.Close())
	}

	return catcher.Resolve()
}

///////////////////
// Helper functions
///////////////////

func parseLogLineString(data string) (LogLine, error) {
	priority, err := strconv.ParseInt(strings.TrimSpace(data[:3]), 10, 16)
	if err != nil {
		return LogLine{}, err
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(data[3:23]), 10, 64)
	if err != nil {
		return LogLine{}, err
	}

	return LogLine{
		Priority:  level.Priority(priority),
		Timestamp: time.Unix(0, ts*1e6).UTC(),
		Data:      data[23:],
	}, nil
}

func prependPriorityAndTimestamp(p level.Priority, t time.Time, data string) string {
	return fmt.Sprintf("%3d%20d%s\n", p, utility.UnixMilli(t), data)
}

func filterChunks(timeRange TimeRange, chunks []LogChunkInfo) []LogChunkInfo {
	filteredChunks := []LogChunkInfo{}
	for i := 0; i < len(chunks); i++ {
		if timeRange.Check(chunks[i].Start) || timeRange.Check(chunks[i].End) {
			filteredChunks = append(filteredChunks, chunks[i])
		}
	}

	return filteredChunks
}

func reverseChunks(chunks []LogChunkInfo) {
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}
}

// LogIteratorHeap is a heap of LogIterator items.
type LogIteratorHeap struct {
	its []LogIterator
	min bool
}

// Len returns the size of the heap.
func (h LogIteratorHeap) Len() int { return len(h.its) }

// Less returns true if the object at index i is less than the object at index
// j in the heap, false otherwise, when min is true. When min is false, the
// opposite is returned.
func (h LogIteratorHeap) Less(i, j int) bool {
	if h.min {
		return h.its[i].Item().Timestamp.Before(h.its[j].Item().Timestamp)
	} else {
		return h.its[i].Item().Timestamp.After(h.its[j].Item().Timestamp)
	}
}

// Swap swaps the objects at indexes i and j.
func (h LogIteratorHeap) Swap(i, j int) { h.its[i], h.its[j] = h.its[j], h.its[i] }

// Push appends a new object of type LogIterator to the heap. Note that if x is
// not a LogIterator nothing happens.
func (h *LogIteratorHeap) Push(x interface{}) {
	it, ok := x.(LogIterator)
	if !ok {
		return
	}

	h.its = append(h.its, it)
}

// Pop returns the next object (as an empty interface) from the heap. Note that
// if the heap is empty this will panic.
func (h *LogIteratorHeap) Pop() interface{} {
	old := h.its
	n := len(old)
	x := old[n-1]
	h.its = old[0 : n-1]
	return x
}

// SafePush is a wrapper function around heap.Push that ensures, during compile
// time, that the correct type of object is put in the heap.
func (h *LogIteratorHeap) SafePush(it LogIterator) {
	heap.Push(h, it)
}

// SafePop is a wrapper function around heap.Pop that converts the returned
// interface into a LogIterator object before returning it.
func (h *LogIteratorHeap) SafePop() LogIterator {
	if h.Len() == 0 {
		return nil
	}

	i := heap.Pop(h)
	it := i.(LogIterator)
	return it
}

/////////////////////////
// Reader Implementations
/////////////////////////

// LogIteratorReaderOptions describes the options for creating a
// LogIteratorReader.
type LogIteratorReaderOptions struct {
	// Limit limits the number of lines read from the log. If equal to 0,
	// lines will be read until the iterator is exhausted. If TailN is
	// greater than 0, Limit will be ignored.
	Limit int
	// TailN is the number of lines to read from the tail of the log. If
	// equal to 0, the reader returned will read log lines in normal order.
	TailN int
	// PrintTime, when true, prints the timestamp of each log line along
	// with the line in the following format:
	//		[2006/01/02 15:04:05.000] This is a log line.
	PrintTime bool
	// PrintPriority, when true, prints the priority of each log line along
	// with the line in the following format:
	//		[P: 30] This is a log line.
	// If PrintTime is also set to true, priority will be printed first:
	//		[P:100] [2006/01/02 15:04:05.000] This is a log line.
	PrintPriority bool
	// SoftSizeLimit assists with pagination of long logs. When set the
	// reader will attempt to read as close to the limit as possible while
	// also reading every line for each timestamp reached. If TailN is set,
	// this will be ignored.
	SoftSizeLimit int
}

// NewLogIteratorReader returns an io.Reader that reads the log lines from the
// log iterator.
func NewLogIteratorReader(ctx context.Context, it LogIterator, opts LogIteratorReaderOptions) io.Reader {
	if opts.TailN > 0 {
		if !it.IsReversed() {
			it = it.Reverse()
		}

		return &logIteratorTailReader{
			ctx:           ctx,
			it:            it,
			n:             opts.TailN,
			printTime:     opts.PrintTime,
			printPriority: opts.PrintPriority,
		}
	}

	return &logIteratorReader{
		ctx:           ctx,
		it:            it,
		limit:         opts.Limit,
		printTime:     opts.PrintTime,
		printPriority: opts.PrintPriority,
		softSizeLimit: opts.SoftSizeLimit,
	}
}

type logIteratorReader struct {
	ctx            context.Context
	it             LogIterator
	lineCount      int
	limit          int
	leftOver       []byte
	printTime      bool
	printPriority  bool
	softSizeLimit  int
	totalBytesRead int
	lastItem       LogLine
}

func (r *logIteratorReader) Read(p []byte) (int, error) {
	n := 0

	if r.leftOver != nil {
		data := r.leftOver
		r.leftOver = nil
		n = r.writeToBuffer(data, p, n)
		if n == len(p) {
			return n, nil
		}
	}

	for r.it.Next(r.ctx) {
		r.lineCount++
		if r.limit > 0 && r.lineCount > r.limit {
			break
		}
		if r.softSizeLimit > 0 && r.totalBytesRead >= r.softSizeLimit && !r.lastItem.Timestamp.Equal(r.it.Item().Timestamp) {
			break
		}

		r.lastItem = r.it.Item()
		data := r.it.Item().Data
		if r.printTime {
			data = fmt.Sprintf("[%s] %s",
				r.it.Item().Timestamp.Format("2006/01/02 15:04:05.000"), data)
		}
		if r.printPriority {
			data = fmt.Sprintf("[P:%3d] %s", r.it.Item().Priority, data)
		}
		n = r.writeToBuffer([]byte(data), p, n)
		if n == len(p) {
			return n, nil
		}
	}

	catcher := grip.NewBasicCatcher()
	catcher.Add(r.it.Err())
	catcher.Add(r.it.Close())
	if catcher.HasErrors() {
		return n, catcher.Resolve()
	}

	return n, io.EOF
}

func (r *logIteratorReader) writeToBuffer(data, buffer []byte, n int) int {
	if len(buffer) == 0 {
		return 0
	}

	m := len(data)
	if n+m > len(buffer) {
		m = len(buffer) - n
		r.leftOver = data[m:]
	}
	_ = copy(buffer[n:n+m], data[:m])

	r.totalBytesRead += m

	return n + m
}

type logIteratorTailReader struct {
	ctx           context.Context
	it            LogIterator
	n             int
	printTime     bool
	printPriority bool
	r             io.Reader
}

func (r *logIteratorTailReader) Read(p []byte) (int, error) {
	if r.r == nil {
		if err := r.getReader(); err != nil {
			return 0, errors.Wrap(err, "problem reading data")
		}
	}

	if len(p) == 0 {
		return 0, io.EOF
	}

	return r.r.Read(p)
}

func (r *logIteratorTailReader) getReader() error {
	var lines string
	for i := 0; i < r.n && r.it.Next(r.ctx); i++ {
		data := r.it.Item().Data
		if r.printTime {
			data = fmt.Sprintf("[%s] %s",
				r.it.Item().Timestamp.Format("2006/01/02 15:04:05.000"), data)
		}
		if r.printPriority {
			data = fmt.Sprintf("[P:%3d] %s", r.it.Item().Priority, data)
		}
		lines = data + lines
	}

	catcher := grip.NewBasicCatcher()
	catcher.Add(r.it.Err())
	catcher.Add(r.it.Close())

	r.r = strings.NewReader(lines)

	return catcher.Resolve()
}

type reverseLineReader struct {
	r     *bufio.Reader
	lines []string
	i     int
}

func newReverseLineReader(r io.Reader) *reverseLineReader {
	return &reverseLineReader{r: bufio.NewReader(r)}
}

func (r *reverseLineReader) ReadLine() (string, error) {
	if r.lines == nil {
		if err := r.getLines(); err != nil {
			return "", errors.Wrap(err, "problem reading lines")
		}
	}

	r.i--
	if r.i < 0 {
		return "", io.EOF
	}

	return r.lines[r.i], nil
}

func (r *reverseLineReader) getLines() error {
	r.lines = []string{}

	for {
		p, err := r.r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.WithStack(err)
		}

		r.lines = append(r.lines, p)
	}

	r.i = len(r.lines)

	return nil
}
