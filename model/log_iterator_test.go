package model

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serialized    = "SerializedIterator"
	serializedR   = "ReversedSerializedIterator"
	batched       = "BatchedIterator"
	batchedR      = "ReversedBatchedIterator"
	parallelized  = "ParallelizedIterator"
	parallelizedR = "ReversedParallelizedIterator"
	merging       = "MergingIterator"
	mergingR      = "ReversedMergingIterator"
)

func TestLogIterator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir(".", "log-iterator-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	bucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)
	chunks, lines, err := GenerateTestLog(ctx, bucket, 99, 30)
	require.NoError(t, err)

	badBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir, Prefix: "DNE"})
	require.NoError(t, err)

	completeTimeRange := TimeRange{EndAt: chunks[len(chunks)-1].End}
	partialTimeRange := TimeRange{
		StartAt: chunks[1].Start,
		EndAt:   chunks[len(chunks)-1].End.Add(-time.Millisecond),
	}
	offset := chunks[0].NumLines
	partialLinesLen := len(lines) - chunks[0].NumLines - 1

	for _, test := range []struct {
		name      string
		iterators map[string]LogIterator
		test      func(*testing.T, string, LogIterator)
	}{
		{
			name: "EmptyIterator",
			iterators: map[string]LogIterator{
				serialized:    NewSerializedLogIterator(bucket, []LogChunkInfo{}, completeTimeRange),
				serializedR:   NewSerializedLogIterator(bucket, []LogChunkInfo{}, completeTimeRange).Reverse(),
				batched:       NewBatchedLogIterator(bucket, []LogChunkInfo{}, 2, completeTimeRange),
				batchedR:      NewBatchedLogIterator(bucket, []LogChunkInfo{}, 2, completeTimeRange).Reverse(),
				parallelized:  NewParallelizedLogIterator(bucket, []LogChunkInfo{}, completeTimeRange),
				parallelizedR: NewParallelizedLogIterator(bucket, []LogChunkInfo{}, completeTimeRange).Reverse(),
				merging:       NewMergingIterator(NewSerializedLogIterator(bucket, []LogChunkInfo{}, completeTimeRange)),
				mergingR:      NewMergingIterator(NewSerializedLogIterator(bucket, []LogChunkInfo{}, completeTimeRange)).Reverse(),
			},
			test: func(t *testing.T, _ string, it LogIterator) {
				assert.False(t, it.Next(ctx))
				assert.True(t, it.Exhausted())
				assert.NoError(t, it.Err())
				assert.Zero(t, it.Item())
				assert.NoError(t, it.Close())
			},
		},
		{
			name: "ExhaustedIterator",
			iterators: map[string]LogIterator{
				serialized:    NewSerializedLogIterator(bucket, chunks, completeTimeRange),
				serializedR:   NewSerializedLogIterator(bucket, chunks, completeTimeRange).Reverse(),
				batched:       NewBatchedLogIterator(bucket, chunks, 2, completeTimeRange),
				batchedR:      NewBatchedLogIterator(bucket, chunks, 2, completeTimeRange).Reverse(),
				parallelized:  NewParallelizedLogIterator(bucket, chunks, completeTimeRange),
				parallelizedR: NewParallelizedLogIterator(bucket, chunks, completeTimeRange).Reverse(),
				merging:       NewMergingIterator(NewBatchedLogIterator(bucket, chunks, 2, completeTimeRange)),
				mergingR:      NewMergingIterator(NewBatchedLogIterator(bucket, chunks, 2, completeTimeRange)).Reverse(),
			},
			test: func(t *testing.T, name string, it LogIterator) {
				var count int
				var i int
				var inc int
				if strings.HasPrefix(name, "Reversed") {
					i = len(lines) - 1
					inc = -1
				} else {
					i = 0
					inc = 1
				}

				for it.Next(ctx) {
					require.True(t, i < len(lines) && i >= 0)
					require.Equal(t, lines[i], it.Item())
					i += inc
					count++
				}
				assert.Equal(t, len(lines), count)
				assert.True(t, it.Exhausted())
				assert.NoError(t, it.Err())
				assert.NoError(t, it.Close())
			},
		},
		{
			name: "ErroredIterator",
			iterators: map[string]LogIterator{
				serialized:   NewSerializedLogIterator(badBucket, chunks, completeTimeRange),
				batched:      NewBatchedLogIterator(badBucket, chunks, 2, completeTimeRange),
				parallelized: NewParallelizedLogIterator(badBucket, chunks, completeTimeRange),
				merging:      NewMergingIterator(NewBatchedLogIterator(badBucket, chunks, 2, completeTimeRange)),
			},
			test: func(t *testing.T, _ string, it LogIterator) {
				count := 0
				for it.Next(ctx) {
					count++
				}
				assert.Equal(t, 0, count)
				assert.False(t, it.Exhausted())
				assert.Error(t, it.Err())
				assert.NoError(t, it.Close())
			},
		},
		{
			name: "LimitedTimeRange",
			iterators: map[string]LogIterator{
				serialized:    NewSerializedLogIterator(bucket, chunks, partialTimeRange),
				serializedR:   NewSerializedLogIterator(bucket, chunks, partialTimeRange).Reverse(),
				batched:       NewBatchedLogIterator(bucket, chunks, 2, partialTimeRange),
				batchedR:      NewBatchedLogIterator(bucket, chunks, 2, partialTimeRange).Reverse(),
				parallelized:  NewParallelizedLogIterator(bucket, chunks, partialTimeRange),
				parallelizedR: NewParallelizedLogIterator(bucket, chunks, partialTimeRange).Reverse(),
				merging:       NewMergingIterator(NewParallelizedLogIterator(bucket, chunks, partialTimeRange)),
				mergingR:      NewMergingIterator(NewParallelizedLogIterator(bucket, chunks, partialTimeRange)).Reverse(),
			},
			test: func(t *testing.T, name string, it LogIterator) {
				var count int
				var i int
				var inc int
				if strings.HasPrefix(name, "Reversed") {
					i = offset + partialLinesLen - 1
					inc = -1
				} else {
					i = offset
					inc = 1
				}

				for it.Next(ctx) {
					require.True(t, i < len(lines) && i >= 0)
					require.Equal(t, lines[i], it.Item())
					i += inc
					count++
				}
				assert.Equal(t, partialLinesLen, count)
				assert.True(t, it.Exhausted())
				assert.NoError(t, it.Err())
				assert.NoError(t, it.Close())
			},
		},
		{
			name: "ReverseAlreadyUsedIterator",
			iterators: map[string]LogIterator{
				serialized:   NewSerializedLogIterator(bucket, chunks, completeTimeRange),
				batched:      NewBatchedLogIterator(bucket, chunks, 2, completeTimeRange),
				parallelized: NewParallelizedLogIterator(bucket, chunks, completeTimeRange),
				merging:      NewMergingIterator(NewBatchedLogIterator(bucket, chunks, 2, completeTimeRange)),
			},
			test: func(t *testing.T, name string, it LogIterator) {
				_ = it.Next(ctx)
				assert.NoError(t, it.Close())
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for impl, it := range test.iterators {
				t.Run(impl, func(t *testing.T) {
					test.test(t, impl, it)
				})
			}
		})
	}
}

func TestMergeLogIterator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "merge-logs-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	bucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	t.Run("SingleLog", func(t *testing.T) {
		chunks, lines, err := GenerateTestLog(ctx, bucket, 100, 10)
		require.NoError(t, err)
		timeRange := TimeRange{
			StartAt: chunks[0].Start,
			EndAt:   chunks[len(chunks)-1].End,
		}
		it := NewMergingIterator(NewBatchedLogIterator(bucket, chunks, 100, timeRange))

		count := 0
		for it.Next(ctx) {
			logLine := it.Item()
			require.True(t, count < len(lines))
			require.Equal(t, lines[count], logLine)
			count++
		}
		assert.Equal(t, len(lines), count)
		assert.NoError(t, it.Err())
		assert.NoError(t, it.Close())
	})
	t.Run("MultipleLogs", func(t *testing.T) {
		numLogs := 10
		its := make([]LogIterator, numLogs)
		lineMap := map[string]bool{}
		for i := 0; i < numLogs; i++ {
			chunks, lines, err := GenerateTestLog(ctx, bucket, 100, 10)
			require.NoError(t, err)

			timeRange := TimeRange{
				StartAt: chunks[0].Start,
				EndAt:   chunks[len(chunks)-1].End,
			}
			its[i] = NewBatchedLogIterator(bucket, chunks, 100, timeRange)
			for _, line := range lines {
				lineMap[line.Data] = false
			}
		}
		it := NewMergingIterator(its...)

		count := 0
		lastTime := time.Time{}
		for it.Next(ctx) {
			logLine := it.Item()

			require.True(t, lastTime.Before(logLine.Timestamp) || lastTime.Equal(logLine.Timestamp))
			lastTime = logLine.Timestamp
			seen, ok := lineMap[logLine.Data]
			require.True(t, ok)
			require.False(t, seen)
			lineMap[logLine.Data] = true
			count++
		}
		assert.Equal(t, len(lineMap), count)
		assert.NoError(t, it.Err())
		assert.NoError(t, it.Close())
	})
	t.Run("MultipleLogsReverse", func(t *testing.T) {
		numLogs := 2
		its := make([]LogIterator, numLogs)
		lineMap := map[string]bool{}
		for i := 0; i < numLogs; i++ {
			chunks, lines, err := GenerateTestLog(ctx, bucket, 100, 10)
			require.NoError(t, err)

			timeRange := TimeRange{
				StartAt: chunks[0].Start,
				EndAt:   chunks[len(chunks)-1].End,
			}
			its[i] = NewBatchedLogIterator(bucket, chunks, 100, timeRange)
			for _, line := range lines {
				lineMap[line.Data] = false
			}
		}
		it := NewMergingIterator(its...).Reverse()

		count := 0
		lastTime := time.Now().Add(365 * 24 * time.Hour)
		for it.Next(ctx) {
			logLine := it.Item()
			require.True(t, lastTime.After(logLine.Timestamp) || lastTime.Equal(logLine.Timestamp))
			lastTime = logLine.Timestamp
			seen, ok := lineMap[logLine.Data]
			require.True(t, ok)
			require.False(t, seen)
			lineMap[logLine.Data] = true
			count++
		}
		assert.Equal(t, len(lineMap), count)
		assert.NoError(t, it.Err())
		assert.NoError(t, it.Close())
	})
	t.Run("SomeExhausted", func(t *testing.T) {
		chunks, _, err := GenerateTestLog(ctx, bucket, 100, 10)
		require.NoError(t, err)
		timeRange := TimeRange{
			StartAt: chunks[0].Start,
			EndAt:   chunks[len(chunks)-1].End,
		}
		it1 := NewBatchedLogIterator(bucket, chunks, 100, timeRange)
		it2 := NewBatchedLogIterator(bucket, chunks, 100, timeRange)
		for it2.Next(ctx) {
			// exhaust
		}
		require.True(t, it2.Exhausted())
		it := NewMergingIterator(it1, it2)

		assert.False(t, it.Exhausted())
		for it.Next(ctx) {
			// exhaust
		}
		assert.True(t, it.Exhausted())
	})
}

func TestLogIteratorReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "merge-logs-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	bucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	chunks, lines, err := GenerateTestLog(ctx, bucket, 100, 10)
	require.NoError(t, err)
	expectedSize := 0
	expectedSizeWithTime := 0
	expectedSizeWithLimit := 0
	expectedSizeWithPriority := 0
	expectedSizeWithPriorityAndTime := 0
	for i, line := range lines {
		expectedSize += len(line.Data)
		formattedTime := line.Timestamp.Format("2006/01/02 15:04:05.000")
		expectedLine := fmt.Sprintf("[%s] %s", formattedTime, line.Data)
		expectedSizeWithTime += len(expectedLine)
		expectedLine = fmt.Sprintf("[P:%3d] %s", line.Priority, line.Data)
		expectedSizeWithPriority += len(expectedLine)
		expectedLine = fmt.Sprintf("[P:%3d] [%s] %s", line.Priority, formattedTime, line.Data)
		expectedSizeWithPriorityAndTime += len(expectedLine)
		if i < 40 {
			expectedSizeWithLimit += len(line.Data)
		}
	}
	timeRange := TimeRange{
		StartAt: chunks[0].Start,
		EndAt:   chunks[len(chunks)-1].End,
	}

	t.Run("LeftOver", func(t *testing.T) {
		opts := LogIteratorReaderOptions{}
		r := NewLogIteratorReader(ctx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		nTotal := 0
		readData := []byte{}
		p := make([]byte, 22)
		for {
			n, err := r.Read(p)
			nTotal += n
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		assert.Equal(t, expectedSize, nTotal)

		current := 0
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			require.Equal(t, lines[current].Data, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithTime", func(t *testing.T) {
		opts := LogIteratorReaderOptions{PrintTime: true}
		r := NewLogIteratorReader(ctx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		nTotal := 0
		readData := []byte{}
		p := make([]byte, 22)
		for {
			n, err := r.Read(p)
			nTotal += n
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		assert.Equal(t, expectedSizeWithTime, nTotal)

		current := 0
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			formattedTime := lines[current].Timestamp.Format("2006/01/02 15:04:05.000")
			expectedLine := fmt.Sprintf("[%s] %s", formattedTime, lines[current].Data)
			require.Equal(t, expectedLine, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithPriority", func(t *testing.T) {
		opts := LogIteratorReaderOptions{PrintPriority: true}
		r := NewLogIteratorReader(ctx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		nTotal := 0
		readData := []byte{}
		p := make([]byte, 22)
		for {
			n, err := r.Read(p)
			nTotal += n
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		assert.Equal(t, expectedSizeWithPriority, nTotal)

		current := 0
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			expectedLine := fmt.Sprintf("[P:%3d] %s", lines[current].Priority, lines[current].Data)
			require.Equal(t, expectedLine, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithPriorityAndTime", func(t *testing.T) {
		opts := LogIteratorReaderOptions{PrintTime: true, PrintPriority: true}
		r := NewLogIteratorReader(ctx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		nTotal := 0
		readData := []byte{}
		p := make([]byte, 22)
		for {
			n, err := r.Read(p)
			nTotal += n
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		assert.Equal(t, expectedSizeWithPriorityAndTime, nTotal)

		current := 0
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			formattedTime := lines[current].Timestamp.Format("2006/01/02 15:04:05.000")
			expectedLine := fmt.Sprintf("[P:%3d] [%s] %s", lines[current].Priority, formattedTime, lines[current].Data)
			require.Equal(t, expectedLine, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithLimit", func(t *testing.T) {
		opts := LogIteratorReaderOptions{Limit: 40}
		r := NewLogIteratorReader(ctx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		nTotal := 0
		readData := []byte{}
		p := make([]byte, 22)
		for {
			n, err := r.Read(p)
			nTotal += n
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		assert.Equal(t, expectedSizeWithLimit, nTotal)

		current := 0
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			require.Equal(t, lines[current].Data, line+"\n")
			current++
		}
		assert.Equal(t, 40, current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithSoftSizeLimit", func(t *testing.T) {
		opts := LogIteratorReaderOptions{SoftSizeLimit: 5000}
		it := NewMergingIterator(
			NewBatchedLogIterator(bucket, chunks, 2, timeRange),
			NewBatchedLogIterator(bucket, chunks, 2, timeRange),
			NewBatchedLogIterator(bucket, chunks, 2, timeRange),
		)
		r := NewLogIteratorReader(ctx, it, opts)
		nTotal := 0
		readData := []byte{}
		p := make([]byte, 22)
		for {
			n, err := r.Read(p)
			nTotal += n
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		assert.Equal(t, 5000+100+51, nTotal) // 51 lines each 100 characters long + newline

		current := 0
		totalLines := 0
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			require.Equal(t, lines[current].Data, line+"\n")
			totalLines++
			if totalLines > 2 && totalLines%3 == 0 {
				current++
			}
		}
		assert.Equal(t, 51, totalLines)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("EmptyBuffer", func(t *testing.T) {
		opts := LogIteratorReaderOptions{}
		r := NewLogIteratorReader(ctx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		p := make([]byte, 0)
		for {
			n, err := r.Read(p)
			require.Zero(t, n)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("ContextError", func(t *testing.T) {
		errCtx, errCancel := context.WithCancel(context.Background())
		errCancel()

		opts := LogIteratorReaderOptions{}
		r := NewLogIteratorReader(errCtx, NewBatchedLogIterator(bucket, chunks, 2, timeRange), opts)
		p := make([]byte, 101)
		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
}

func TestLogIteratorTailReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir("", "merge-logs-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	bucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	chunks, lines, err := GenerateTestLog(ctx, bucket, 100, 10)
	require.NoError(t, err)
	timeRange := TimeRange{
		StartAt: chunks[0].Start,
		EndAt:   chunks[len(chunks)-1].End,
	}

	t.Run("NLines", func(t *testing.T) {
		it := NewBatchedLogIterator(bucket, chunks, 2, timeRange)
		opts := LogIteratorReaderOptions{Limit: 20, TailN: 42, SoftSizeLimit: 1}
		r := NewLogIteratorReader(ctx, it, opts)
		readData := []byte{}
		p := make([]byte, 4096)
		for {
			n, err := r.Read(p)
			readData = append(readData, p[:n]...)
			assert.True(t, n >= 0)
			assert.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		current := 58
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			require.Equal(t, lines[current].Data, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithTime", func(t *testing.T) {
		it := NewBatchedLogIterator(bucket, chunks, 2, timeRange)
		opts := LogIteratorReaderOptions{TailN: 42, PrintTime: true}
		r := NewLogIteratorReader(ctx, it, opts)
		readData := []byte{}
		p := make([]byte, 4096)
		for {
			n, err := r.Read(p)
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		current := 58
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			formattedTime := lines[current].Timestamp.Format("2006/01/02 15:04:05.000")
			expectedLine := fmt.Sprintf("[%s] %s", formattedTime, lines[current].Data)
			require.Equal(t, expectedLine, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithPriority", func(t *testing.T) {
		it := NewBatchedLogIterator(bucket, chunks, 2, timeRange)
		opts := LogIteratorReaderOptions{TailN: 42, PrintPriority: true}
		r := NewLogIteratorReader(ctx, it, opts)
		readData := []byte{}
		p := make([]byte, 4096)
		for {
			n, err := r.Read(p)
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		current := 58
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			expectedLine := fmt.Sprintf("[P:%3d] %s", lines[current].Priority, lines[current].Data)
			require.Equal(t, expectedLine, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("WithPriorityAndTime", func(t *testing.T) {
		it := NewBatchedLogIterator(bucket, chunks, 2, timeRange)
		opts := LogIteratorReaderOptions{
			TailN:         42,
			PrintTime:     true,
			PrintPriority: true,
		}
		r := NewLogIteratorReader(ctx, it, opts)
		readData := []byte{}
		p := make([]byte, 4096)
		for {
			n, err := r.Read(p)
			readData = append(readData, p[:n]...)
			require.True(t, n >= 0)
			require.True(t, n <= len(p))
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		current := 58
		readLines := strings.Split(string(readData), "\n")
		assert.Equal(t, "", readLines[len(readLines)-1])
		for _, line := range readLines[:len(readLines)-1] {
			require.True(t, current < len(lines))
			formattedTime := lines[current].Timestamp.Format("2006/01/02 15:04:05.000")
			expectedLine := fmt.Sprintf("[P:%3d] [%s] %s", lines[current].Priority, formattedTime, lines[current].Data)
			require.Equal(t, expectedLine, line+"\n")
			current++
		}
		assert.Equal(t, len(lines), current)

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("EmptyBuffer", func(t *testing.T) {
		opts := LogIteratorReaderOptions{TailN: 42}
		it := NewBatchedLogIterator(bucket, chunks, 2, timeRange)
		r := NewLogIteratorReader(ctx, it, opts)
		p := make([]byte, 0)
		for {
			n, err := r.Read(p)
			require.Zero(t, n)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
	t.Run("ContextError", func(t *testing.T) {
		errCtx, errCancel := context.WithCancel(context.Background())
		errCancel()

		opts := LogIteratorReaderOptions{TailN: 10}
		it := NewBatchedLogIterator(bucket, chunks, 2, timeRange)
		r := NewLogIteratorReader(errCtx, it, opts)
		p := make([]byte, 101)
		n, err := r.Read(p)
		assert.Zero(t, n)
		assert.Error(t, err)
	})
}
