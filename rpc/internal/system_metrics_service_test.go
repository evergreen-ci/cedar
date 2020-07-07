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
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

func TestAddSystemMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createSystemMetricsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownSystemMetricsEnv(ctx, env))
	}()
	tempDir, err := ioutil.TempDir(".", "system-metrics-test")
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	require.NoError(t, err)

	conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
	require.NoError(t, err)

	sm := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test"}, model.PailLocal)
	sm.Setup(env)
	require.NoError(t, sm.SaveNew(ctx))

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: sm.Artifact.Prefix,
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
				LogId: sm.ID,
				Lines: []*LogLine{
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second system metrics data chunk.\n",
					},
					{
						Priority:  10,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third system metrics data chunk.\n",
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
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third system metrics data chunk.\n",
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidTimestamp",
			lines: &LogLines{
				LogId: sm.ID,
				Lines: []*LogLine{
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: 253402300800},
						Data:      "This is the third system metrics data chunk, which is invalid.\n",
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidEnv",
			lines: &LogLines{
				LogId: sm.ID,
				Lines: []*LogLine{
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third system metrics data chunk.\n",
					},
				},
			},
			env:    nil,
			hasErr: true,
		},
		{
			name: "InvalidConf",
			lines: &LogLines{
				LogId: sm.ID,
				Lines: []*LogLine{
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the first system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the second system metrics data chunk.\n",
					},
					{
						Priority:  30,
						Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
						Data:      "This is the third system metrics data chunk.\n",
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
			require.NoError(t, startSystemMetricsService(ctx, test.env, port))
			client, err := getSystemMetricsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			if test.invalidConf {
				conf.Bucket.BuildLogsBucket = ""
			} else {
				conf.Bucket.BuildLogsBucket = tempDir
			}
			conf.Setup(env)
			require.NoError(t, conf.Save())

			resp, err := client.AddSystemMetrics(ctx, test.lines)
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
				assert.Equal(t, sm.ID, sm.Info.ID())
				assert.Len(t, l.Artifact.Chunks, 1)
				_, err := bucket.Get(ctx, l.Artifact.Chunks[0].Key)
				assert.NoError(t, err)
			}
		})
	}
}

func TestStreamSystemMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createSystemMetricsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownSystemMetricsEnv(ctx, env))
	}()
	tempDir, err := ioutil.TempDir(".", "system-metrics-test")
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	require.NoError(t, err)

	conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
	require.NoError(t, err)

	sm := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test"}, model.PailLocal)
	sm.Setup(env)
	require.NoError(t, sm.SaveNew(ctx))
	log2 := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test2"}, model.PailLocal)
	log2.Setup(env)
	require.NoError(t, log2.SaveNew(ctx))

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: sm.Artifact.Prefix,
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
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the first system metrics data chunk.\n",
						},
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the second system metrics data chunk.\n",
						},
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the third system metrics data chunk.\n",
						},
					},
				},
				{
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the fourth system metrics data chunk.\n",
						},
					},
				},
				{
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the fifth system metrics data chunk.\n",
						},
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the sixth system metrics data chunk.\n",
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
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the first system metrics data chunk.\n",
						},
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the second system metrics data chunk.\n",
						},
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the third system metrics data chunk.\n",
						},
					},
				},
				{
					LogId: log2.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the fourth system metrics data chunk.\n",
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
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the first system metrics data chunk.\n",
						},
					},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidTimestamp",
			lines: []*LogLines{
				{
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: 253402300800},
							Data:      "This is the third system metrics data chunk, which is invalid.\n",
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
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the first system metrics data chunk.\n",
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
					LogId: sm.ID,
					Lines: []*LogLine{
						{
							Priority:  30,
							Timestamp: &timestamp.Timestamp{Seconds: time.Now().Unix()},
							Data:      "This is the first system metrics data chunk.\n",
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
			require.NoError(t, startSystemMetricsService(ctx, test.env, port))
			client, err := getSystemMetricsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			if test.invalidConf {
				conf.Bucket.BuildLogsBucket = ""
			} else {
				conf.Bucket.BuildLogsBucket = tempDir
			}
			conf.Setup(env)
			require.NoError(t, conf.Save())

			stream, err := client.StreamSystemMetrics(ctx)
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
				assert.Equal(t, sm.ID, sm.Info.ID())
				assert.Len(t, l.Artifact.Chunks, len(test.lines))
				for _, chunk := range l.Artifact.Chunks {
					_, err := bucket.Get(ctx, chunk.Key)
					assert.NoError(t, err)
				}
			}
		})
	}
}

func createSystemMetricsEnv() (cedar.Environment, error) {
	testDB := "system-metrics-service-test"
	env, err := cedar.NewEnvironment(context.Background(), testDB, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})

	return env, err
}

func teardownSystemMetricsEnv(ctx context.Context, env cedar.Environment) error {
	return errors.WithStack(env.GetDB().Drop(ctx))
}

func startSystemMetricsService(ctx context.Context, env cedar.Environment, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachSystemMetricsService(env, s)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func getSystemMetricsGRPCClient(ctx context.Context, address string, opts []grpc.DialOption) (CedarSystemMetricsClient, error) {
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return NewCedarSystemMetricsClient(conn), nil
}
