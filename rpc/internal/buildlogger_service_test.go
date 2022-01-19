package internal

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCreateLog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createBuildloggerEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownBuildloggerEnv(ctx, env))
	}()

	for _, test := range []struct {
		name   string
		data   *LogData
		env    cedar.Environment
		hasErr bool
	}{
		{
			name: "ValidData",
			data: &LogData{
				Info:    &LogInfo{Project: "test"},
				Storage: LogStorage_LOG_STORAGE_S3,
			},
			env: env,
		},
		{
			name: "InvalidEnv",
			data: &LogData{
				Info:    &LogInfo{Project: "test3"},
				Storage: LogStorage_LOG_STORAGE_GRIDFS,
			},
			env:    nil,
			hasErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
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
				assert.Error(t, log.Find(ctx))
			} else {
				require.NoError(t, err)
				assert.Equal(t, info.ID(), resp.LogId)

				log := &model.Log{ID: resp.LogId}
				log.Setup(env)
				require.NoError(t, log.Find(ctx))
				assert.Equal(t, info, log.Info)
				assert.Equal(t, test.data.Storage.Export(), log.Artifact.Type)
				assert.True(t, time.Since(log.CreatedAt) <= time.Second)
			}
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

	conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
	require.NoError(t, err)

	log := model.CreateLog(model.LogInfo{Project: "test"}, model.PailLocal)
	log.Setup(env)
	require.NoError(t, log.SaveNew(ctx))

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: log.Artifact.Prefix,
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
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
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the first log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the second log line.\n"),
					},
					{
						Priority:  10,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the third log line.\n"),
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
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the first log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the second log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the third log line.\n"),
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
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the first log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the second log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the third log line.\n"),
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
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the first log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the second log line.\n"),
					},
					{
						Priority:  30,
						Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
						Data:      []byte("This is the third log line.\n"),
					},
				},
			},
			env:         env,
			invalidConf: true,
			hasErr:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
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
				require.NoError(t, l.Find(ctx))
				assert.Equal(t, log.ID, log.Info.ID())
				iter, err := bucket.List(ctx, "")
				assert.NoError(t, err)
				var chunkCount int
				for iter.Next(ctx) {
					chunkCount++
				}
				assert.Equal(t, 1, chunkCount)
			}
		})
	}
}

func TestStreamLogLines(t *testing.T) {
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

	conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
	require.NoError(t, err)

	log := model.CreateLog(model.LogInfo{Project: "test"}, model.PailLocal)
	log.Setup(env)
	require.NoError(t, log.SaveNew(ctx))
	log2 := model.CreateLog(model.LogInfo{Project: "test2"}, model.PailLocal)
	log2.Setup(env)
	require.NoError(t, log2.SaveNew(ctx))

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: log.Artifact.Prefix,
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
		lines       []*LogLines
		env         cedar.Environment
		invalidConf bool
		hasErr      bool
	}{
		{
			name: "ValidData",
			lines: []*LogLines{
				{
					LogId: log.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the first log line.\n"),
						},
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the second log line.\n"),
						},
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the third log line.\n"),
						},
					},
				},
				{
					LogId: log.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the fourth log line.\n"),
						},
					},
				},
				{
					LogId: log.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the fifth log line.\n"),
						},
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the sixth log line.\n"),
						},
					},
				},
			},
			env: env,
		},
		{
			name: "DifferentLogIDs",
			lines: []*LogLines{
				{
					LogId: log.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the first log line.\n"),
						},
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the second log line.\n"),
						},
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the third log line.\n"),
						},
					},
				},
				{
					LogId: log2.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the fourth log line.\n"),
						},
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "LogDNE",
			lines: []*LogLines{
				{
					LogId: "DNE",
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the first log line.\n"),
						},
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidEnv",
			lines: []*LogLines{
				{
					LogId: log.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the first log line.\n"),
						},
					},
				},
			},
			env:    nil,
			hasErr: true,
		},
		{
			name: "InvalidConf",
			lines: []*LogLines{
				{
					LogId: log.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
							Data:      []byte("This is the first log line.\n"),
						},
					},
				},
			},
			env:         env,
			invalidConf: true,
			hasErr:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
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

			stream, err := client.StreamLogLines(ctx)
			require.NoError(t, err)

			catcher := grip.NewBasicCatcher()
			for i := 0; i < len(test.lines); i++ {
				catcher.Add(stream.Send(test.lines[i]))
			}
			resp, err := stream.CloseAndRecv()
			catcher.Add(err)

			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, test.lines[0].LogId, resp.LogId)

				l := &model.Log{ID: resp.LogId}
				l.Setup(env)
				require.NoError(t, l.Find(ctx))
				assert.Equal(t, log.ID, log.Info.ID())
				iter, err := bucket.List(ctx, "")
				assert.NoError(t, err)
				var chunkCount int
				for iter.Next(ctx) {
					chunkCount++
				}
				assert.Equal(t, len(test.lines), chunkCount)
			}
		})
	}
}

func TestCloseLog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createBuildloggerEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownBuildloggerEnv(ctx, env))
	}()

	log := model.CreateLog(model.LogInfo{Project: "test"}, model.PailLocal)
	log.Setup(env)
	require.NoError(t, log.SaveNew(ctx))

	for _, test := range []struct {
		name   string
		info   *LogEndInfo
		env    cedar.Environment
		hasErr bool
	}{
		{
			name: "ValidData",
			info: &LogEndInfo{
				LogId:    log.ID,
				ExitCode: 1,
			},
			env: env,
		},
		{
			name: "DefaultData",
			info: &LogEndInfo{LogId: log.ID},
			env:  env,
		},
		{
			name:   "LogDNE",
			info:   &LogEndInfo{LogId: "DNE"},
			env:    env,
			hasErr: true,
		},
		{
			name:   "InvalidEnv",
			info:   &LogEndInfo{LogId: log.ID},
			env:    nil,
			hasErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			require.NoError(t, startBuildloggerService(ctx, test.env, port))
			client, err := getBuildloggerGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			resp, err := client.CloseLog(ctx, test.info)
			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, log.ID, resp.LogId)

				l := &model.Log{ID: resp.LogId}
				l.Setup(env)
				require.NoError(t, l.Find(ctx))
				assert.Equal(t, log.ID, l.Info.ID())
				assert.Equal(t, log.Artifact, l.Artifact)
				assert.Equal(t, int(test.info.ExitCode), l.Info.ExitCode)
				assert.True(t, time.Since(l.CompletedAt) <= time.Second)
			}
		})
	}
}

func createBuildloggerEnv() (cedar.Environment, error) {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
		DisableCache:  true,
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
