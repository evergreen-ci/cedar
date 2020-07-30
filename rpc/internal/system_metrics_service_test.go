package internal

import (
	"context"
	fmt "fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

func TestCreateSystemMetricsRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createSystemMetricsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownSystemMetricsEnv(ctx, env))
	}()

	port := getPort()
	require.NoError(t, startSystemMetricsService(ctx, env, port))
	client, err := getSystemMetricsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
	require.NoError(t, err)
	port = getPort()
	require.NoError(t, startSystemMetricsService(ctx, nil, port))
	invalidClient, err := getSystemMetricsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
	require.NoError(t, err)

	t.Run("NoConfig", func(t *testing.T) {
		info := getSystemMetricsInfo()
		artifact := getSystemMetricsArtifactInfo()
		modelInfo := info.Export()
		systemMetrics := &SystemMetrics{
			Info:     info,
			Artifact: artifact,
		}

		resp, err := client.CreateSystemMetricsRecord(ctx, systemMetrics)
		assert.Error(t, err)
		assert.Nil(t, resp)

		sm := &model.SystemMetrics{ID: modelInfo.ID()}
		sm.Setup(env)
		assert.Error(t, sm.Find(ctx))
	})

	conf := model.NewCedarConfig(env)
	require.NoError(t, conf.Save())
	t.Run("InvalidEnv", func(t *testing.T) {
		info := getSystemMetricsInfo()
		artifact := getSystemMetricsArtifactInfo()
		modelInfo := info.Export()
		systemMetrics := &SystemMetrics{
			Info:     info,
			Artifact: artifact,
		}

		resp, err := invalidClient.CreateSystemMetricsRecord(ctx, systemMetrics)
		assert.Error(t, err)
		assert.Nil(t, resp)

		sm := &model.SystemMetrics{ID: modelInfo.ID()}
		sm.Setup(env)
		assert.Error(t, sm.Find(ctx))
	})
	t.Run("ConfigWithoutBucketType", func(t *testing.T) {
		info := getSystemMetricsInfo()
		artifact := getSystemMetricsArtifactInfo()
		modelInfo := info.Export()
		systemMetrics := &SystemMetrics{
			Info:     info,
			Artifact: artifact,
		}

		resp, err := client.CreateSystemMetricsRecord(ctx, systemMetrics)
		assert.Error(t, err)
		assert.Nil(t, resp)

		sm := &model.SystemMetrics{ID: modelInfo.ID()}
		sm.Setup(env)
		assert.Error(t, sm.Find(ctx))

	})
	conf.Bucket.SystemMetricsBucketType = model.PailS3
	require.NoError(t, conf.Save())
	t.Run("ConfigWithBucketType", func(t *testing.T) {
		info := getSystemMetricsInfo()
		artifact := getSystemMetricsArtifactInfo()
		modelInfo := info.Export()
		systemMetrics := &SystemMetrics{
			Info:     info,
			Artifact: artifact,
		}

		resp, err := client.CreateSystemMetricsRecord(ctx, systemMetrics)
		require.NoError(t, err)
		require.NotNil(t, resp)

		sm := &model.SystemMetrics{ID: modelInfo.ID()}
		sm.Setup(env)
		require.NoError(t, sm.Find(ctx))
		assert.Equal(t, modelInfo.ID(), resp.Id)
		assert.Equal(t, modelInfo, sm.Info)
		assert.Equal(t, conf.Bucket.SystemMetricsBucketType, sm.Artifact.Options.Type)
		assert.Equal(t, model.FileUncompressed, sm.Artifact.Options.Compression)
		assert.Equal(t, model.SchemaRawEvents, sm.Artifact.Options.Schema)
		assert.True(t, time.Since(sm.CreatedAt) <= time.Second)
	})
}

func TestAddSystemMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createSystemMetricsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownSystemMetricsEnv(ctx, env))
	}()
	tempDir, err := ioutil.TempDir(".", "system-metrics-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
	require.NoError(t, err)

	systemMetrics := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test"}, model.SystemMetricsArtifactOptions{
		Type: model.PailLocal,
	})
	systemMetrics.Setup(env)
	require.NoError(t, systemMetrics.SaveNew(ctx))

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: systemMetrics.Artifact.Prefix,
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
		chunk       *SystemMetricsData
		env         cedar.Environment
		invalidConf bool
		hasErr      bool
	}{
		{
			name: "ValidData",
			chunk: &SystemMetricsData{
				Id:     systemMetrics.ID,
				Type:   "Test",
				Format: DataFormat_FTDC,
				Data:   []byte("Byte chunk for valid data"),
			},
			env: env,
		},
		{
			name: "LogDNE",
			chunk: &SystemMetricsData{
				Id:     "DNE",
				Type:   "Test",
				Format: DataFormat_FTDC,
				Data:   []byte("Byte chunk when id doesn't exist"),
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidEnv",
			chunk: &SystemMetricsData{
				Id:     systemMetrics.ID,
				Type:   "Test",
				Format: DataFormat_FTDC,
				Data:   []byte("Byte chunk with no env"),
			},
			env:    nil,
			hasErr: true,
		},
		{
			name: "InvalidConf",
			chunk: &SystemMetricsData{
				Id:     systemMetrics.ID,
				Type:   "Test",
				Format: DataFormat_FTDC,
				Data:   []byte("Byte chunk with no conf"),
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
				conf.Bucket.SystemMetricsBucket = ""
			} else {
				conf.Bucket.SystemMetricsBucket = tempDir
			}
			conf.Setup(env)
			require.NoError(t, conf.Save())

			resp, err := client.AddSystemMetrics(ctx, test.chunk)
			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, test.chunk.Id, resp.Id)

				sm := &model.SystemMetrics{ID: resp.Id}
				sm.Setup(env)
				require.NoError(t, sm.Find(ctx))
				assert.Equal(t, systemMetrics.ID, systemMetrics.Info.ID())
				require.Contains(t, sm.Artifact.MetricChunks, "Test")
				assert.Len(t, sm.Artifact.MetricChunks["Test"].Chunks, 1)
				assert.Equal(t, sm.Artifact.MetricChunks["Test"].Format, model.FileFTDC)
				_, err := bucket.Get(ctx, sm.Artifact.MetricChunks["Test"].Chunks[0])
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

	systemMetrics := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test"}, model.SystemMetricsArtifactOptions{
		Type: model.PailLocal,
	})
	systemMetrics.Setup(env)
	require.NoError(t, systemMetrics.SaveNew(ctx))
	systemMetrics2 := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test2"}, model.SystemMetricsArtifactOptions{
		Type: model.PailLocal,
	})
	systemMetrics2.Setup(env)
	require.NoError(t, systemMetrics2.SaveNew(ctx))

	bucket, err := pail.NewLocalBucket(pail.LocalOptions{
		Path:   tempDir,
		Prefix: systemMetrics.Artifact.Prefix,
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
		chunks      []*SystemMetricsData
		env         cedar.Environment
		invalidConf bool
		hasErr      bool
	}{
		{
			name: "ValidData",
			chunks: []*SystemMetricsData{
				{
					Id:     systemMetrics.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("First byte chunk for valid data"),
				},
				{
					Id:     systemMetrics.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("Second byte chunk for valid data"),
				},
				{
					Id:     systemMetrics.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("Third byte chunk for valid data"),
				},
			},
			env: env,
		},
		{
			name: "DifferentSystemMetricsIDs",
			chunks: []*SystemMetricsData{
				{
					Id:     systemMetrics.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("First byte chunk for different system metrics ids"),
				},
				{
					Id:     systemMetrics2.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("Second byte chunk for different system metrics ids"),
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "SystemMetricsDNE",
			chunks: []*SystemMetricsData{
				{
					Id:     "DNE",
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("First byte chunk for invalid system metrics ids"),
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidEnv",
			chunks: []*SystemMetricsData{
				{
					Id:     systemMetrics.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("First byte chunk for invalid env"),
				},
			},
			env:    nil,
			hasErr: true,
		},
		{
			name: "InvalidConf",
			chunks: []*SystemMetricsData{
				{
					Id:     systemMetrics.ID,
					Type:   "Test",
					Format: DataFormat_FTDC,
					Data:   []byte("First byte chunk for invalid conf"),
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
				conf.Bucket.SystemMetricsBucket = ""
			} else {
				conf.Bucket.SystemMetricsBucket = tempDir
			}
			conf.Setup(env)
			require.NoError(t, conf.Save())

			stream, err := client.StreamSystemMetrics(ctx)
			require.NoError(t, err)

			catcher := grip.NewBasicCatcher()
			for i := 0; i < len(test.chunks); i++ {
				catcher.Add(stream.Send(test.chunks[i]))
			}
			resp, err := stream.CloseAndRecv()
			catcher.Add(err)

			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, test.chunks[0].Id, resp.Id)

				sm := &model.SystemMetrics{ID: resp.Id}
				sm.Setup(env)
				require.NoError(t, sm.Find(ctx))
				assert.Equal(t, systemMetrics.ID, systemMetrics.Info.ID())
				require.Contains(t, sm.Artifact.MetricChunks, "Test")
				assert.Len(t, sm.Artifact.MetricChunks["Test"].Chunks, len(test.chunks))
				assert.Equal(t, sm.Artifact.MetricChunks["Test"].Format, model.FileFTDC)

				for _, key := range sm.Artifact.MetricChunks["Test"].Chunks {
					_, err := bucket.Get(ctx, key)
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestCloseMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createSystemMetricsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownSystemMetricsEnv(ctx, env))
	}()

	systemMetrics := model.CreateSystemMetrics(model.SystemMetricsInfo{Project: "test"}, model.SystemMetricsArtifactOptions{
		Type: model.PailLocal,
	})
	systemMetrics.Setup(env)
	require.NoError(t, systemMetrics.SaveNew(ctx))

	for _, test := range []struct {
		name   string
		info   *SystemMetricsSeriesEnd
		env    cedar.Environment
		hasErr bool
	}{
		{
			name: "ValidData",
			info: &SystemMetricsSeriesEnd{
				Id:      systemMetrics.ID,
				Success: true,
			},
			env: env,
		},
		{
			name:   "SystemMetricsDNE",
			info:   &SystemMetricsSeriesEnd{Id: "DNE"},
			env:    env,
			hasErr: true,
		},
		{
			name:   "InvalidEnv",
			info:   &SystemMetricsSeriesEnd{Id: systemMetrics.ID},
			env:    nil,
			hasErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			require.NoError(t, startSystemMetricsService(ctx, test.env, port))
			client, err := getSystemMetricsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			resp, err := client.CloseMetrics(ctx, test.info)
			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, systemMetrics.ID, resp.Id)

				sm := &model.SystemMetrics{ID: resp.Id}
				sm.Setup(env)
				require.NoError(t, sm.Find(ctx))
				assert.Equal(t, systemMetrics.ID, sm.Info.ID())
				assert.True(t, time.Since(sm.CreatedAt) <= time.Second)
				assert.Equal(t, test.info.Success, sm.Info.Success)
			}
		})
	}
}

func createSystemMetricsEnv() (cedar.Environment, error) {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
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

func getSystemMetricsInfo() *SystemMetricsInfo {
	return &SystemMetricsInfo{
		Project:   utility.RandomString(),
		Version:   utility.RandomString(),
		Variant:   utility.RandomString(),
		TaskName:  utility.RandomString(),
		TaskId:    utility.RandomString(),
		Execution: rand.Int31n(5),
		Mainline:  false,
	}
}

func getSystemMetricsArtifactInfo() *SystemMetricsArtifactInfo {
	return &SystemMetricsArtifactInfo{
		Compression: CompressionType_NONE,
		Schema:      SchemaType_RAW_EVENTS,
	}
}
