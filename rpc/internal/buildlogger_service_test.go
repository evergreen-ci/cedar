package internal

import (
	"context"
	fmt "fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

func TestCreateLog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createBuildloggerEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownBuildloggerEnv(ctx, env))
	}()
	port := 4000

	for _, test := range []struct {
		name        string
		data        *LogData
		env         cedar.Environment
		ts          time.Time
		nilResponse bool
		hasErr      bool
	}{
		{
			name: "ValidData",
			data: &LogData{
				Info:      &LogInfo{Project: "test"},
				Storage:   LogStorage_LOG_STORAGE_S3,
				CreatedAt: &timestamp.Timestamp{Seconds: 253402300799},
			},
			ts:  time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC),
			env: env,
		},
		{
			name: "DefaultTimestamp",
			data: &LogData{
				Info:      &LogInfo{Project: "test2"},
				Storage:   LogStorage_LOG_STORAGE_LOCAL,
				CreatedAt: nil,
			},
			ts:  time.Now().UTC(),
			env: env,
		},
		{
			name: "InvalidTimestamp",
			data: &LogData{
				Info:      &LogInfo{Project: "test3"},
				Storage:   LogStorage_LOG_STORAGE_GRIDFS,
				CreatedAt: &timestamp.Timestamp{Seconds: 253402300800},
			},
			env:         env,
			nilResponse: true,
			hasErr:      true,
		},
		{
			name: "InvalidEnv",
			data: &LogData{
				Info:      &LogInfo{Project: "test3"},
				Storage:   LogStorage_LOG_STORAGE_GRIDFS,
				CreatedAt: &timestamp.Timestamp{},
			},
			env:    nil,
			hasErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, startBuildloggerService(ctx, test.env, port))
			client, err := getBuildloggerGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			info := test.data.Info.Export()
			resp, err := client.CreateLog(ctx, test.data)
			if test.hasErr {
				assert.Nil(t, resp)
				assert.Error(t, err)

				log := &model.Log{ID: info.ID()}
				log.Setup(env)
				assert.Error(t, log.Find())
			} else {
				require.NoError(t, err)
				assert.Equal(t, info.ID(), resp.LogId)

				log := &model.Log{ID: resp.LogId}
				log.Setup(env)
				require.NoError(t, log.Find())
				assert.Equal(t, info, log.Info)
				assert.Equal(t, test.data.Storage.Export(), log.Artifact.Type)
				assert.Equal(t, test.ts.Round(time.Minute), log.CreatedAt.Round(time.Minute))
			}

			port += 1
		})
	}
}

func TestAppendLogLines(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createBuildloggerEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownBuildloggerEnv(ctx, env))
	}()
	tempDir, err := ioutil.TempDir(".", "buildlogger-test")
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	require.NoError(t, err)
	port := 5000

	conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
	require.NoError(t, err)

	log := model.CreateLog(model.LogInfo{Project: "test"}, model.PailLocal)
	log.Setup(env)
	require.NoError(t, log.SaveNew())

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: log.Artifact.Prefix,
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
		setup       func()
		lines       *LogLines
		env         cedar.Environment
		invalidConf bool
		hasErr      bool
	}{
		{
			name: "ValidData",
			lines: &LogLines{
				LogId: log.ID,
				Lines: []*LogLine{
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third log line.\n",
					},
				},
			},
			env: env,
		},
		{
			name: "LogDNE",
			lines: &LogLines{
				LogId: "DNE",
				Lines: []*LogLine{
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third log line.\n",
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidTimestamp",
			lines: &LogLines{
				LogId: log.ID,
				Lines: []*LogLine{
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: 253402300800},
						Data:      "This is the third log line, which is invalid.\n",
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidEnv",
			lines: &LogLines{
				LogId: log.ID,
				Lines: []*LogLine{
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third log line.\n",
					},
				},
			},
			env:    nil,
			hasErr: true,
		},
		{
			name: "InvalidConf",
			lines: &LogLines{
				LogId: log.ID,
				Lines: []*LogLine{
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second log line.\n",
					},
					{
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third log line.\n",
					},
				},
			},
			env:         env,
			invalidConf: true,
			hasErr:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, startBuildloggerService(ctx, test.env, port))
			client, err := getBuildloggerGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			if test.invalidConf {
				conf.Bucket.BuildLogsBucket = ""
			} else {
				conf.Bucket.BuildLogsBucket = tempDir
			}
			conf.Setup(env)
			require.NoError(t, conf.Save())

			resp, err := client.AppendLogLines(ctx, test.lines)
			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, test.lines.LogId, resp.LogId)

				l := &model.Log{ID: resp.LogId}
				l.Setup(env)
				require.NoError(t, l.Find())
				assert.Len(t, l.Artifact.Chunks, 1)
				_, err := bucket.Get(ctx, l.Artifact.Chunks[0].Key)
				assert.NoError(t, err)
			}

			port += 1
		})
	}
}

func createBuildloggerEnv() (cedar.Environment, error) {
	testDB := "buildlogger-service-test"
	env, err := cedar.NewEnvironment(context.Background(), testDB, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})

	return env, err
}

func teardownBuildloggerEnv(ctx context.Context, env cedar.Environment) error {
	return errors.WithStack(env.GetDB().Drop(ctx))
}

func startBuildloggerService(ctx context.Context, env cedar.Environment, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachBuildloggerService(env, s)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func getBuildloggerGRPCClient(ctx context.Context, address string, opts []grpc.DialOption) (BuildloggerClient, error) {
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return NewBuildloggerClient(conn), nil
}
