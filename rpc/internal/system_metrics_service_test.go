package internal

import (
	"context"
	fmt "fmt"
	"net"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

func TestCreateSystemMetricRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createSystemMetricsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownSystemMetricsEnv(ctx, env))
	}()

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
				Id:   systemMetrics.ID,
				Data: []byte("Byte chunk for valid data"),
			},
			env: env,
		},
		{
			name: "InvalidEnv",
			chunk: &SystemMetricsData{
				Id:   systemMetrics.ID,
				Data: []byte("Byte chunk with no env"),
			},
			env:    nil,
			hasErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			require.NoError(t, startSystemMetricsService(ctx, test.env, port))
			client, err := getSystemMetricsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			info := test.data.Info.Export()
			resp, err := client.CreateSystemMetricsRecord(ctx, test.data)
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
