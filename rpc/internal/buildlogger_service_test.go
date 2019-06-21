package internal

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
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
			ts:          time.Now().UTC(),
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
			ts:     time.Now().UTC(),
			env:    nil,
			hasErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			srvCtx, srvCancel := context.WithTimeout(ctx, time.Minute)
			require.NoError(t, startBuildloggerService(srvCtx, test.env))
			client, err := getBuildloggerGRPCClient(srvCtx, localAddress, []grpc.DialOption{grpc.WithInsecure()})
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

			srvCancel()
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

func startBuildloggerService(ctx context.Context, env cedar.Environment) error {
	lis, err := net.Listen("tcp", localAddress)
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
