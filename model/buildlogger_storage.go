package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

// LogFormat is a type that describes the format of a log.
type LogFormat string

// Valid log formats.
const (
	LogFormatUnknown LogFormat = "unknown"
	LogFormatText    LogFormat = "text"
	LogFormatJSON    LogFormat = "json"
	LogFormatBSON    LogFormat = "bson"
)

// Validate the log format.
func (lf LogFormat) Validate() error {
	switch lf {
	case LogFormatUnknown, LogFormatText, LogFormatJSON, LogFormatBSON:
		return nil
	default:
		return errors.New("invalid log format type specified")
	}
}

// LogArtifact describes a bucket of logs stored in some kind of offline blob
// storage. It is the bridge between pail-backed offline log storage and the
// cedar-based log metadata storage. The prefix field indicates the name of the
// "sub-bucket". The top level bucket is accesible via the cedar.Environment
// interface.
type LogArtifactInfo struct {
	Type    PailType       `bson:"type"`
	Prefix  string         `bson:"prefix"`
	Version int            `bson:"version"`
	Chunks  []LogChunkInfo `bson:"chunks,omitempty"`
}

var (
	logArtifactInfoTypeKey    = bsonutil.MustHaveTag(LogArtifactInfo{}, "Type")
	logArtifactInfoPrefixKey  = bsonutil.MustHaveTag(LogArtifactInfo{}, "Prefix")
	logArtifactInfoVersionKey = bsonutil.MustHaveTag(LogArtifactInfo{}, "Version")
	logArtifactInfoChunksKey  = bsonutil.MustHaveTag(LogArtifactInfo{}, "Chunks")
)

// LogChunkInfo describes a chunk of log lines stored in pail-backed offline
// storage.
type LogChunkInfo struct {
	Key      string    `bson:"key"`
	NumLines int       `bson:"num_lines"`
	Start    time.Time `bson:"start"`
	End      time.Time `bson:"end"`
}

var (
	logLogChunkInfoKeyKey      = bsonutil.MustHaveTag(LogChunkInfo{}, "Key")
	logLogChunkInfoNumLinesKey = bsonutil.MustHaveTag(LogChunkInfo{}, "NumLines")
	logLogChunkInfoStartKey    = bsonutil.MustHaveTag(LogChunkInfo{}, "Start")
	logLogChunkInfoEndKey      = bsonutil.MustHaveTag(LogChunkInfo{}, "End")
)

func createBuildloggerChunkKey(start, end time.Time, numLines int) string {
	return fmt.Sprintf("%d_%d_%d", start.UnixNano(), end.UnixNano(), numLines)
}

func parseBuildloggerChunkKey(key string) (LogChunkInfo, error) {
	chunkInfo := strings.Split(key, "_")
	if len(chunkInfo) != 3 {
		return LogChunkInfo{}, errors.New("invalid buildlogger chunk key")
	}

	start, err := strconv.ParseInt(chunkInfo[0], 10, 64)
	if err != nil {
		return LogChunkInfo{}, errors.Wrap(err, "parsing buildlogger chunk start time")
	}
	end, err := strconv.ParseInt(chunkInfo[1], 10, 64)
	if err != nil {
		return LogChunkInfo{}, errors.Wrap(err, "parsing buildlogger chunk end time")
	}
	numLines, err := strconv.Atoi(chunkInfo[2])
	if err != nil {
		return LogChunkInfo{}, errors.Wrap(err, "parsing buildlogger chunk num lines")
	}

	return LogChunkInfo{
		Key:      key,
		NumLines: numLines,
		Start:    time.Unix(0, start).UTC(),
		End:      time.Unix(0, end).UTC(),
	}, nil
}
