package model

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"math"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const buildloggerCollection = "buildlogger"

type JSONLog []*LogLine

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Data      string    `json:"data"`
}

type Log struct {
	ID        string      `bson:"_id"`
	Info      LogInfo     `bson:"info"`
	CreatedAt time.Time   `bson:"created_at"`
	Artifact  LogArtifact `bson:"artifact"`

	env       cedar.Environment
	populated bool
}

func CreateLog(info LogInfo, permissions string) (*Log, error) {
	createdAt := time.Now()

	if err := validatePermissions(permissions); err != nil {
		return nil, errors.Wrap(err, "failed to create new log")
	}

	return &Log{
		ID:        info.ID(),
		Info:      info,
		CreatedAt: createdAt,
		Artifact: LogArtifact{
			Dir:         info.ID(),
			Permissions: permissions,
			Version:     0,
		},
	}, nil
}

func (l *Log) Setup(e cedar.Environment) { l.env = e }
func (l *Log) IsNil() bool               { return !l.populated }
func (l *Log) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	l.populated = false
	err = session.DB(conf.DatabaseName).C(buildloggerCollection).FindId(l.ID).One(l)
	if db.ResultsNotFound(err) {
		return errors.New("could not find log record in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding log")
	}

	l.populated = true

	return nil
}

func (l *Log) Save() error {
	if !l.populated {
		return errors.New("cannot save non-populated log data")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
		if l.ID == "" {
			return errors.New("cannot save log data without ID")
		}
	}

	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(buildloggerCollection).UpsertId(l.ID, l)
	grip.DebugWhen(err == nil, message.Fields{
		"ns":     model.Namespace{DB: conf.DatabaseName, Collection: buildloggerCollection},
		"id":     l.ID,
		"change": changeInfo,
		"op":     "save log",
	})
	return errors.Wrap(err, "problem saving log to collection")
}

func (l *Log) Remove() error {
	if l.ID == "" {
		l.ID = l.Info.ID()
		if l.ID == "" {
			return errors.New("cannot remove log without ID")
		}
	}

	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	if err = session.DB(conf.DatabaseName).C(buildloggerCollection).RemoveId(l.ID); err != nil {
		return errors.Wrap(err, "problem removing log")
	}
	return nil
}

func (l *Log) AppendLines(lines []*LogLine) error {
	if l.ID == "" {
		l.ID = l.Info.ID()
		if l.ID == "" {
			return errors.New("cannot append log lines without ID")
		}
	}

	return nil
}

////////////////////////////////////////////////////////////////////////
//
// Component Types

type LogInfo struct {
	Project    string                 `bson:"project"`
	Version    string                 `bson:"version"`
	Variant    string                 `bson:"variant"`
	TaskName   string                 `bson:"task_name"`
	TaskID     string                 `bson:"task_id"`
	Execution  int                    `bson:"execution"`
	TestName   string                 `bson:"test_name"`
	Trial      int                    `bson:"trial"` // may not need this
	ProcName   string                 `bson:"proc_name"`
	Categories map[string]interface{} `bson:"categories"` // may not need this
	Schema     int                    `bson:"schema"`
	// figure out what other info I should add for more granular logs.
}

func (id *LogInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
		_, _ = io.WriteString(hash, id.Variant)
		_, _ = io.WriteString(hash, id.TaskName)
		_, _ = io.WriteString(hash, id.TaskID)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Execution))
		_, _ = io.WriteString(hash, id.TestName)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Trial))
		_, _ = io.WriteString(hash, fmt.Sprint(id.Schema))

	} else {
		panic("unsupported schema")
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

type LogArtifact struct {
	Dir         string   `bson:"dir"`
	Permissions string   `bson:"permissions"` // defaults to private
	Version     int      `bson:"version"`     // what is version?
	Paths       []string `bson:"paths"`

	mutex sync.Mutex
}

func validatePermissions(permissions string) error {
	switch permissions {
	case s3.ObjectCannedACLPrivate:
		return nil
	case s3.ObjectCannedACLPublicRead:
		return nil
	case s3.ObjectCannedACLPublicReadWrite:
		return nil
	case s3.ObjectCannedACLAuthenticatedRead:
		return nil
	case s3.ObjectCannedACLAwsExecRead:
		return nil
	case s3.ObjectCannedACLBucketOwnerRead:
		return nil
	case s3.ObjectCannedACLBucketOwnerFullControl:
		return nil
	default:
		return errors.Errorf("unrecognized permission type %s", permissions)
	}
}

// These upload functions convert log lines into three different storage formats:
// 	* JSON (using LogLines struct above)
//	* Text with prefix header
// 	* Raw text
// I will write a test to compare the speeds of uploading and downloading the
// log data in these formats. A merge function may need to be written for
// these tests.
func (a *LogArtifact) UploadLogLinesJSON(lines [][]interface{}) error {
	path := a.getPath()
	logLines := []*LogLine{}

	for _, line := range lines {
		// timeField is generated client-side as the output of python's time.time(), which returns
		// seconds since epoch as a floating point number
		timeField, ok := line[0].(float64)
		if !ok {
			grip.Critical(message.Fields{
				"message": "unable to convert time field",
				"value":   line[0],
			})
			timeField = float64(time.Now().Unix())
		}

		// extract fractional seconds from the total time and convert to nanoseconds
		fractionalPart := timeField - math.Floor(timeField)
		nSecPart := int64(fractionalPart * float64(int64(time.Second)/int64(time.Nanosecond)))

		timeParsed := time.Unix(int64(timeField), nSecPart)
		logLines = append(logLines, &LogLine{
			Timestamp: timeParsed,
			Data:      line[1].(string),
		})
	}

	jsonLines, err := json.Marshal(logLines)
	if err != nil {
		return errors.Wrap(err, "problem marshalling lines into json")
	}

	bucket, err := getTempBucket()
	if err != nil {
		return errors.Wrap(err, "problem getting bucket for uploading json lines")
	}

	return errors.Wrap(bucket.Put(context.TODO(), path, bytes.NewReader(jsonLines)), "failed to upload json")
}

func (a *LogArtifact) DownloadLogJSON() ([]*LogLine, error) {
	log := []*LogLine{}

	bucket, err := getTempBucket()
	if err != nil {
		return nil, errors.Wrap(err, "problem getting bucket for downloading json lines")
	}

	for _, path := range a.Paths {
		logLines := &JSONLog{}

		reader, err := bucket.Get(context.TODO(), path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get JSON log lines")
		}

		bytes, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read JSON log lines")
		}

		err = json.Unmarshal(bytes, logLines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal JSON log lines")
		}

		log = append(log, *logLines...)
	}

	return log, nil
}

func (a *LogArtifact) UploadLogLinesPrefix(lines [][]interface{}) error {
	path := a.getPath()
	formattedLines := []byte{}

	for _, line := range lines {
		// timeField is generated client-side as the output of python's time.time(), which returns
		// seconds since epoch as a floating point number
		timeField, ok := line[0].(float64)
		if !ok {
			grip.Critical(message.Fields{
				"message": "unable to convert time field",
				"value":   line[0],
			})
			timeField = float64(time.Now().Unix())
		}

		// extract fractional seconds from the total time and convert to nanoseconds
		fractionalPart := timeField - math.Floor(timeField)
		nSecPart := int64(fractionalPart * float64(int64(time.Second)/int64(time.Nanosecond)))

		timeParsed := time.Unix(int64(timeField), nSecPart)
		formattedLines = append(
			formattedLines,
			[]byte(fmt.Sprintf("%s%s", timeParsed.Format(time.RFC3339), line[1].(string)))...,
		)
	}

	bucket, err := getTempBucket()
	if err != nil {
		return errors.Wrap(err, "problem getting bucket for uploading prefix lines")
	}

	return errors.Wrap(bucket.Put(context.TODO(), path, bytes.NewReader(formattedLines)), "failed to upload prefix lines")
}

func (a *LogArtifact) DownloadLog() ([]byte, error) {
	log := []byte{}

	bucket, err := getTempBucket()
	if err != nil {
		return nil, errors.Wrap(err, "problem getting bucket for downloading json lines")
	}

	for _, path := range a.Paths {
		bytes := []byte{}

		reader, err := bucket.Get(context.TODO(), path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get prefix log lines")
		}

		_, err = ioutil.ReadAll(reader)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read prefix log lines")
		}

		log = append(log, bytes...)
	}

	return log, nil
}

func (a *LogArtifact) UploadLogLinesRaw(lines [][]interface{}) error {
	path := a.getPath()
	rawLines := []byte{}

	bucket, err := getTempBucket()
	if err != nil {
		return errors.Wrap(err, "problem getting bucket for downloading json lines")
	}

	for _, line := range lines {
		rawLines = append(rawLines, line[1].([]byte)...)
	}

	return errors.Wrap(bucket.Put(context.TODO(), path, bytes.NewReader(rawLines)), "failed to upload raw lines")
}

func getTempBucket() (pail.Bucket, error) {
	opts := pail.S3Options{
		Name:       "pail-bucket-test",
		Region:     "us-east-1",
		Permission: "public-read",
	}
	bucket, err := pail.NewS3Bucket(opts)
	return bucket, errors.WithStack(err)
}

func (a *LogArtifact) getPath() string {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	path := filepath.Join(a.Dir, fmt.Sprintf("%s", time.Now()))
	a.Paths = append(a.Paths, path)
	return path
}
