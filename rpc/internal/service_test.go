package internal

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/mongodb/amboy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
	mgo "gopkg.in/mgo.v2"
)

const (
	address = "localhost:50051"
)

type MockEnv struct {
	queue   amboy.Queue
	session *mgo.Session
	conf    *sink.Configuration
}

func (m *MockEnv) Configure(config *sink.Configuration) error {
	m.conf = config
	return nil
}

func (m *MockEnv) GetConf() (*sink.Configuration, error) {
	return m.conf, nil
}

func (m *MockEnv) SetQueue(queue amboy.Queue) error {
	m.queue = queue
	return nil
}

func (m *MockEnv) GetQueue() (amboy.Queue, error) {
	return m.queue, nil
}

func (m *MockEnv) GetSession() (*mgo.Session, error) {
	return m.session, errors.New("mock err")
}

func startPerfService(ctx context.Context, env sink.Environment) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachService(env, s)

	go s.Serve(lis)
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func getClient(ctx context.Context) (SinkPerformanceMetricsClient, error) {
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	return NewSinkPerformanceMetricsClient(conn), nil
}

func createEnv(mock bool) (sink.Environment, error) {
	if mock {
		return &MockEnv{}, nil
	}
	env := sink.GetEnvironment()
	err := env.Configure(&sink.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "grpc_test",
		NumWorkers:    2,
		UseLocalQueue: true,
	})
	return env, errors.WithStack(err)
}

func tearDownEnv(env sink.Environment, mock bool) error {
	if mock {
		return nil
	}
	conf, session, err := sink.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func TestCreate(t *testing.T) {
	for _, test := range []struct {
		name         string
		mockEnv      bool
		function     func(SinkPerformanceMetricsClient, context.Context, *ResultData, ...grpc.CallOption) (*MetricsResponse, error)
		data         *ResultData
		expectedResp *MetricsResponse
		err          bool
	}{
		{
			name:     "TestCreateMetricSeries",
			function: SinkPerformanceMetricsClient.CreateMetricSeries,
			data: &ResultData{
				Id: &ResultID{
					Project: "testProject",
					Version: "testVersion",
				},
			},
			expectedResp: &MetricsResponse{
				Id: (&model.PerformanceResultInfo{
					Project: "testProject",
					Version: "testVersion",
				}).ID(),
				Success: true,
			},
		},
		{
			name:     "TestCreateMetricSeriesWithInvalidData",
			function: SinkPerformanceMetricsClient.CreateMetricSeries,
			data:     &ResultData{},
			err:      true,
		},
		{
			name:     "TestCreateMetricSeriesWithInvalidEnv",
			mockEnv:  true,
			function: SinkPerformanceMetricsClient.CreateMetricSeries,
			data: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, err := createEnv(test.mockEnv)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, tearDownEnv(env, test.mockEnv))
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err = startPerfService(ctx, env)
			require.NoError(t, err)
			client, err := getClient(ctx)
			require.NoError(t, err)

			resp, err := test.function(client, ctx, test.data)
			assert.Equal(t, test.expectedResp, resp)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAttach(t *testing.T) {
	for _, test := range []struct {
		name               string
		function           func(SinkPerformanceMetricsClient, context.Context, *ResultData, ...grpc.CallOption) (*MetricsResponse, error)
		save               bool
		resultData         *ResultData
		attachedResultData *ResultData
		expectedResp       *MetricsResponse
		err                bool
	}{
		{
			name:     "TestAttachResultData",
			function: SinkPerformanceMetricsClient.AttachResultData,
			save:     true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedResultData: &ResultData{
				Id:        &ResultID{},
				Artifacts: []*ArtifactInfo{},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name:               "TestAttachResultDataInvalidData",
			function:           SinkPerformanceMetricsClient.AttachResultData,
			attachedResultData: &ResultData{},
			err:                true,
		},
		{
			name:     "TestAttachResultDataDoesNotExist",
			function: SinkPerformanceMetricsClient.AttachResultData,
			attachedResultData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name:     "TestAttachResultDataInvalidEnv",
			function: SinkPerformanceMetricsClient.AttachResultData,
			attachedResultData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name:     "TestAttachAuxilaryData",
			function: SinkPerformanceMetricsClient.AttachAuxilaryData,
			save:     true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedResultData: &ResultData{
				Id:        &ResultID{},
				Artifacts: []*ArtifactInfo{},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name:               "TestAttachAuxilaryDataInvalidData",
			function:           SinkPerformanceMetricsClient.AttachAuxilaryData,
			attachedResultData: &ResultData{},
			err:                true,
		},
		{
			name:     "TestAttachAuxilaryDataDoesNotExist",
			function: SinkPerformanceMetricsClient.AttachAuxilaryData,
			attachedResultData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name:     "TestAttachAuxilaryDataInvalidEnv",
			function: SinkPerformanceMetricsClient.AttachAuxilaryData,
			attachedResultData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name:     "TestAttachRollups",
			function: SinkPerformanceMetricsClient.AttachRollups,
			save:     true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedResultData: &ResultData{
				Id: &ResultID{},
				Rollups: &Rollups{
					ProcessedAt: ptypes.TimestampNow(),
				},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name:               "TestAttachRollupsInvalidData",
			function:           SinkPerformanceMetricsClient.AttachRollups,
			attachedResultData: &ResultData{},
			err:                true,
		},
		{
			name:     "TestAttachRollupsDoesNotExist",
			function: SinkPerformanceMetricsClient.AttachRollups,
			attachedResultData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name:     "TestAttachRollupsInvalidEnv",
			function: SinkPerformanceMetricsClient.AttachRollups,
			attachedResultData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name:     "TestAttachRollupsInvalidRollup",
			function: SinkPerformanceMetricsClient.AttachRollups,
			save:     true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedResultData: &ResultData{
				Id:      &ResultID{},
				Rollups: &Rollups{},
			},
			err: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, err := createEnv(false)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, tearDownEnv(env, false))
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err = startPerfService(ctx, env)
			require.NoError(t, err)
			client, err := getClient(ctx)
			require.NoError(t, err)

			if test.save {
				_, err := client.CreateMetricSeries(ctx, test.resultData)
				require.NoError(t, err)
			}
			resp, err := test.function(client, ctx, test.attachedResultData)
			assert.Equal(t, test.expectedResp, resp)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
