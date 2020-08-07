package model

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip/level"
	"github.com/pkg/errors"
)

// GenerateTestLog is a convenience function to generate random logs with 100
// character long lines of the given size and chunk size in the given bucket.
func GenerateTestLog(ctx context.Context, bucket pail.Bucket, size, chunkSize int) ([]LogChunkInfo, []LogLine, error) {
	lines := make([]LogLine, size)
	numChunks := size / chunkSize
	if numChunks == 0 || size%chunkSize > 0 {
		numChunks += 1
	}
	chunks := make([]LogChunkInfo, numChunks)
	ts := time.Now().Round(time.Millisecond).UTC()

	for i := 0; i < numChunks; i++ {
		rawLines := ""
		chunks[i] = LogChunkInfo{
			Key:   newRandCharSetString(16),
			Start: ts,
		}

		j := 0
		for j < chunkSize && j+i*chunkSize < size {
			line := newRandCharSetString(100)
			lines[j+i*chunkSize] = LogLine{
				Priority:  level.Debug,
				Timestamp: ts,
				Data:      line + "\n",
			}
			rawLines += prependPriorityAndTimestamp(level.Debug, ts, line)
			ts = ts.Add(time.Millisecond)
			j++
		}

		if err := bucket.Put(ctx, chunks[i].Key, strings.NewReader(rawLines)); err != nil {
			return []LogChunkInfo{}, []LogLine{}, errors.Wrap(err, "failed to add chunk to bucket")
		}

		chunks[i].NumLines = j
		chunks[i].End = ts.Add(-time.Millisecond)
		ts = ts.Add(time.Hour)
	}

	return chunks, lines, nil
}

// GenerateSystemMetrics is a convenience function to generate a specified number of
// 16 byte random system metrics data chunks in the specified bucket.
func GenerateSystemMetrics(ctx context.Context, bucket pail.Bucket, num int) ([]string, map[string][]byte, error) {
	keys := []string{}
	dataChunks := map[string][]byte{}

	for i := 0; i < num; i++ {
		key := fmt.Sprintf("chunk-%d", i)
		keys = append(keys, key)
		data := []byte(utility.RandomString())
		dataChunks[key] = data
		if err := bucket.Put(ctx, key, bytes.NewReader(data)); err != nil {
			return []string{}, map[string][]byte{}, errors.Wrap(err, "problem creating system metrics chunks")
		}
	}

	return keys, dataChunks, nil
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func newRandCharSetString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
