package internal

import (
	"context"
	fmt "fmt"
	"github.com/golang/protobuf/ptypes/timestamp"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

const (
	localAddress  = "localhost:2289"
	remoteAddress = "cedar.mongodb.com:7070"
	testDBName    = "cedar_grpc_test"
)

func init() {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	if err != nil {
		panic(err)
	}
	cedar.SetEnvironment(env)
}

func startPerfService(ctx context.Context, env cedar.Environment, port int, proxyCreator func(options model.ProxyServiceOptions) perf.ProxyService) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachPerfService(env, s, proxyCreator)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func getGRPCClient(ctx context.Context, address string, opts []grpc.DialOption) (CedarPerformanceMetricsClient, error) {
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return NewCedarPerformanceMetricsClient(conn), nil
}

func tearDownEnv(env cedar.Environment, mock bool) error {
	if mock {
		return nil
	}
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func checkRollups(t *testing.T, ctx context.Context, env cedar.Environment, id string, rollups []*RollupValue) {
	q := env.GetRemoteQueue()
	require.NotNil(t, q)
	amboy.WaitInterval(ctx, q, 250*time.Millisecond)
	for j := range q.Results(ctx) {
		err := j.Error()
		msg := "job completed without error"
		if err != nil {
			msg = err.Error()
		}

		grip.Info(message.Fields{
			"job":     j.ID(),
			"result":  id,
			"message": msg,
		})
	}

	conf, sess, err := cedar.GetSessionWithConfig(env)
	require.NoError(t, err)
	result := &model.PerformanceResult{}
	assert.NoError(t, sess.DB(conf.DatabaseName).C("perf_results").FindId(id).One(result))
	if len(result.Artifacts) > 0 {
		assert.True(t, len(result.Rollups.Stats) > len(rollups), "%s", id)
	} else {
		assert.True(t, len(result.Rollups.Stats) == len(rollups), "%s", id)
	}

	rollupMap := map[string]model.PerfRollupValue{}
	for _, rollup := range result.Rollups.Stats {
		rollupMap[rollup.Name] = rollup
	}
	for _, rollup := range rollups {
		actualRollup, ok := rollupMap[rollup.Name]
		require.True(t, ok)

		var expectedValue interface{}
		if x, ok := rollup.Value.(*RollupValue_Int); ok {
			expectedValue = x.Int
		} else if x, ok := rollup.Value.(*RollupValue_Fl); ok {
			expectedValue = x.Fl
		}
		assert.Equal(t, expectedValue, actualRollup.Value)
		assert.Equal(t, int(rollup.Version), actualRollup.Version)
		assert.Equal(t, rollup.UserSubmitted, actualRollup.UserSubmitted)
	}
}

func TestCreateMetricSeries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range []struct {
		name         string
		mockEnv      bool
		data         *ResultData
		expectedResp *MetricsResponse
		err          bool
	}{
		{
			name: "TestValidDataMainline",
			data: &ResultData{
				Id: &ResultID{
					Project:  "testProjectMainline",
					Version:  "testVersionMainline",
					Mainline: true,
				},
				Artifacts: []*ArtifactInfo{
					{
						Location:  5,
						Bucket:    "testdata",
						Path:      "valid.ftdc",
						CreatedAt: &timestamp.Timestamp{},
					},
				},
				Rollups: []*RollupValue{
					{
						Name:    "Max",
						Value:   &RollupValue_Int{Int: 5},
						Type:    0,
						Version: 1,
					},
				},
			},
			expectedResp: &MetricsResponse{
				Id: (&model.PerformanceResultInfo{
					Project: "testProjectMainline",
					Version: "testVersionMainline",
				}).ID(),
				Success: true,
			},
		},
		{
			name: "TestValidDataPatch",
			data: &ResultData{
				Id: &ResultID{
					Project:  "testProjectPatch",
					Version:  "testVersionPatch",
					Mainline: false,
				},
				Rollups: []*RollupValue{
					{
						Name:    "Max",
						Value:   &RollupValue_Int{Int: 5},
						Type:    0,
						Version: 1,
					},
				},
			},
			expectedResp: &MetricsResponse{
				Id: (&model.PerformanceResultInfo{
					Project: "testProjectPatch",
					Version: "testVersionPatch",
				}).ID(),
				Success: true,
			},
		},
		{
			name: "TestInvalidData",
			data: &ResultData{},
			err:  true,
		},
		{
			name:    "TestInvalidEnv",
			mockEnv: true,
			data: &ResultData{
				Id: &ResultID{Mainline: true},
			},
			err: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			var env cedar.Environment
			if !test.mockEnv {
				env = cedar.GetEnvironment()
				conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
				require.NoError(t, err)
				conf.Setup(env)
				require.NoError(t, conf.Save())
			}
			defer func() {
				require.NoError(t, tearDownEnv(env, test.mockEnv))
			}()
			var mockProxyService = &perf.MockProxyService{}

			require.NoError(t, startPerfService(ctx, env, port, perf.NewMockProxyServiceCreator(mockProxyService)))
			client, err := getGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			resp, err := client.CreateMetricSeries(ctx, test.data)
			require.Equal(t, test.expectedResp, resp)
			if test.data.Id != nil && !test.data.Id.Mainline {
				require.Equal(t, len(mockProxyService.Calls), 1)
				require.Equal(t, test.data.Id.Project, mockProxyService.Calls[0].Project)
				require.Equal(t, test.data.Id.Version, mockProxyService.Calls[0].Version)
				require.Equal(t, test.data.Id.TaskId, mockProxyService.Calls[0].TaskID)
				require.Equal(t, test.data.Id.Variant, mockProxyService.Calls[0].Variant)
				require.Equal(t, test.data.Id.TaskName, mockProxyService.Calls[0].Task)
				require.Equal(t, test.data.Id.TestName, mockProxyService.Calls[0].Test)
				require.Equal(t, test.data.Id.Arguments, mockProxyService.Calls[0].Arguments)
			}

			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				checkRollups(t, ctx, env, resp.Id, test.data.Rollups)
			}
		})
	}
}

func TestAttachResultData(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range []struct {
		name         string
		save         bool
		resultData   *ResultData
		attachedData interface{}
		expectedResp *MetricsResponse
		err          bool
		checkRollups bool
	}{
		{
			name: "TestAttachArtifacts",
			save: true,
			resultData: &ResultData{
				Id: &ResultID{Mainline: true},
			},
			attachedData: &ArtifactData{
				Id: (&model.PerformanceResultInfo{}).ID(),
				Artifacts: []*ArtifactInfo{
					{
						Location:  5,
						Bucket:    "testdata",
						Path:      "valid.ftdc",
						CreatedAt: &timestamp.Timestamp{},
					},
				},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
			checkRollups: true,
		},
		{
			name: "TestAttachArtifactsDoesNotExist",
			attachedData: &ArtifactData{
				Id: (&model.PerformanceResultInfo{}).ID(),
				Artifacts: []*ArtifactInfo{
					{
						Location:  5,
						Bucket:    "testdata",
						Path:      "valid.ftdc",
						CreatedAt: &timestamp.Timestamp{},
					},
				},
			},
			err: true,
		},
		{
			name: "TestAttachRollups",
			save: true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedData: &RollupData{
				Id: (&model.PerformanceResultInfo{}).ID(),
				Rollups: []*RollupValue{
					{
						Name:    "rollup1",
						Version: 1,
					},
					{
						Name:    "rollup2",
						Version: 1,
					},
				},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name: "TestAttachRollupsDoesNotExist",
			attachedData: &RollupData{
				Id: (&model.PerformanceResultInfo{}).ID(),
			},
			err: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				require.NoError(t, tearDownEnv(env, false))
			}()
			port := getPort()
			var mockProxyService = &perf.MockProxyService{}

			conf, err := model.LoadCedarConfig(filepath.Join("testdata", "cedarconf.yaml"))
			require.NoError(t, err)
			conf.Setup(env)
			require.NoError(t, conf.Save())

			require.NoError(t, startPerfService(ctx, env, port, perf.NewMockProxyServiceCreator(mockProxyService)))
			client, err := getGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			if test.save {
				_, err = client.CreateMetricSeries(ctx, test.resultData)
				require.NoError(t, err)
			}

			var resp *MetricsResponse
			switch d := test.attachedData.(type) {
			case *ArtifactData:
				resp, err = client.AttachArtifacts(ctx, d)
			case *RollupData:
				resp, err = client.AttachRollups(ctx, d)
				if test.resultData != nil && test.resultData.Id != nil && !test.resultData.Id.Mainline {
					require.Equal(t, len(mockProxyService.Calls), 1)
					require.Equal(t, test.resultData.Id.Project, mockProxyService.Calls[0].Project)
					require.Equal(t, test.resultData.Id.Version, mockProxyService.Calls[0].Version)
					require.Equal(t, test.resultData.Id.TaskId, mockProxyService.Calls[0].TaskID)
					require.Equal(t, test.resultData.Id.Variant, mockProxyService.Calls[0].Variant)
					require.Equal(t, test.resultData.Id.TaskName, mockProxyService.Calls[0].Task)
					require.Equal(t, test.resultData.Id.TestName, mockProxyService.Calls[0].Test)
					require.Equal(t, test.resultData.Id.Arguments, mockProxyService.Calls[0].Arguments)
				}
			default:
				t.Error("unknown attached data type")
			}

			assert.Equal(t, test.expectedResp, resp)

			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if test.checkRollups {
				checkRollups(t, ctx, env, resp.Id, nil)
			}
		})
	}
}
