package testutils

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip/level"
	"github.com/pkg/errors"
)

func CreateLog(ctx context.Context, bucket pail.Bucket, size, chunkSize int) ([]model.LogChunkInfo, []model.LogLine, error) {
	lines := make([]model.LogLine, size)
	numChunks := size / chunkSize
	if numChunks == 0 || size%chunkSize > 0 {
		numChunks += 1
	}
	chunks := make([]model.LogChunkInfo, numChunks)
	ts := time.Now().Round(time.Millisecond).UTC()

	for i := 0; i < numChunks; i++ {
		rawLines := ""
		chunks[i] = model.LogChunkInfo{
			Key:   newRandCharSetString(16),
			Start: ts,
		}

		j := 0
		for j < chunkSize && j+i*chunkSize < size {
			line := newRandCharSetString(100)
			lines[j+i*chunkSize] = model.LogLine{
				Priority:  level.Debug,
				Timestamp: ts,
				Data:      line + "\n",
			}
			rawLines += fmt.Sprintf("%3d%20d%s\n", level.Debug, util.UnixMilli(ts), line)
			ts = ts.Add(time.Minute)
			j++
		}

		if err := bucket.Put(ctx, chunks[i].Key, strings.NewReader(rawLines)); err != nil {
			return []model.LogChunkInfo{}, []model.LogLine{}, errors.Wrap(err, "failed to add chunk to bucket")
		}

		chunks[i].NumLines = j
		chunks[i].End = ts.Add(-time.Minute)
		ts = ts.Add(time.Hour)
	}

	return chunks, lines, nil
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
