package internal

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/mongodb/amboy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
	mgo "gopkg.in/mgo.v2"
)

const (
	localAddress  = "localhost:50051"
	remoteAddress = "cedar.mongodb.com:7070"
)

type MockEnv struct {
	queue   amboy.Queue
	session *mgo.Session
	conf    *cedar.Configuration
}

func (m *MockEnv) Configure(config *cedar.Configuration) error {
	m.conf = config
	return nil
}

func (m *MockEnv) GetConf() (*cedar.Configuration, error) {
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

func startPerfService(ctx context.Context, env cedar.Environment) error {
	lis, err := net.Listen("tcp", localAddress)
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachService(env, s)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func getClient(ctx context.Context) (CedarPerformanceMetricsClient, error) {
	conn, err := grpc.DialContext(ctx, localAddress, grpc.WithInsecure())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return NewCedarPerformanceMetricsClient(conn), nil
}

func createEnv(mock bool) (cedar.Environment, error) {
	if mock {
		return &MockEnv{}, nil
	}
	env := cedar.GetEnvironment()
	err := env.Configure(&cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "grpc_test",
		SocketTimeout: time.Hour,
		NumWorkers:    2,
		UseLocalQueue: true,
	})
	return env, errors.WithStack(err)
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

func TestCreateMetricSeries(t *testing.T) {
	for _, test := range []struct {
		name         string
		mockEnv      bool
		data         *ResultData
		expectedResp *MetricsResponse
		err          bool
	}{
		{
			name: "TestValidData",
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
			name: "TestInvalidData",
			data: &ResultData{},
			err:  true,
		},
		{
			name:    "TestInvalidEnv",
			mockEnv: true,
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

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			err = startPerfService(ctx, env)
			require.NoError(t, err)
			client, err := getClient(ctx)
			require.NoError(t, err)

			resp, err := client.CreateMetricSeries(ctx, test.data)
			assert.Equal(t, test.expectedResp, resp)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAttachResultData(t *testing.T) {
	for _, test := range []struct {
		name         string
		save         bool
		resultData   *ResultData
		attachedData interface{}
		expectedResp *MetricsResponse
		err          bool
	}{
		{
			name: "TestAttachResultData",
			save: true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedData: &ResultData{
				Id:        &ResultID{},
				Artifacts: []*ArtifactInfo{},
				Rollups:   []*RollupValue{},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name: "TestAttachResultDataWithEmptyFields",
			save: true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedData: &ResultData{
				Id: &ResultID{},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name:         "TestAttachResultDataInvalidData",
			attachedData: &ResultData{},
			err:          true,
		},
		{
			name: "TestAttachResultDataDoesNotExist",
			attachedData: &ResultData{
				Id: &ResultID{},
			},
			err: true,
		},
		{
			name: "TestAttachArtifacts",
			save: true,
			resultData: &ResultData{
				Id: &ResultID{},
			},
			attachedData: &ArtifactData{
				Id:        (&model.PerformanceResultInfo{}).ID(),
				Artifacts: []*ArtifactInfo{},
			},
			expectedResp: &MetricsResponse{
				Id:      (&model.PerformanceResultInfo{}).ID(),
				Success: true,
			},
		},
		{
			name: "TestAttachArtifactsDoesNotExist",
			attachedData: &ArtifactData{
				Id: (&model.PerformanceResultInfo{}).ID(),
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
			env, err := createEnv(false)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, tearDownEnv(env, false))
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			err = startPerfService(ctx, env)
			require.NoError(t, err)
			client, err := getClient(ctx)
			require.NoError(t, err)

			if test.save {
				_, err = client.CreateMetricSeries(ctx, test.resultData)
				require.NoError(t, err)
			}

			var resp *MetricsResponse
			switch d := test.attachedData.(type) {
			case *ResultData:
				resp, err = client.AttachResultData(ctx, d)
			case *ArtifactData:
				resp, err = client.AttachArtifacts(ctx, d)
			case *RollupData:
				resp, err = client.AttachRollups(ctx, d)
			default:
				t.Error("unknown attached data type")
			}
			assert.Equal(t, test.expectedResp, resp)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCuratorSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	caCert := filepath.Join("testdata", "cedar.ca")
	userCert := filepath.Join("testdata", "user.crt")
	userKey := filepath.Join("testdata", "user.key")
	expectedResult := model.CreatePerformanceResult(
		model.PerformanceResultInfo{
			Project:   "sys-perf",
			Variant:   "linux-standalone",
			TaskName:  "smoke_test",
			TaskID:    "abcd",
			TestName:  "not_a_real_test_name",
			Execution: 1,
			Trial:     1,
		},
		[]model.ArtifactInfo{},
	)
	curatorPath, err := filepath.Abs("curator")
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		skip   bool
		setUp  func(t *testing.T) *exec.Cmd
		closer func(t *testing.T)
	}{
		{
			name: "WithoutAuthOrTLS",
			setUp: func(t *testing.T) *exec.Cmd {
				env, err := createEnv(false)
				assert.NoError(t, err)
				assert.NoError(t, startPerfService(ctx, env))

				return createSendCommand(curatorPath, localAddress, "", "", "", true)
			},
			closer: func(t *testing.T) {
				env := cedar.GetEnvironment()
				defer func() {
					require.NoError(t, tearDownEnv(env, false))
				}()

				conf, session, err := cedar.GetSessionWithConfig(env)
				require.NoError(t, err)
				perfResult := &model.PerformanceResult{}
				assert.NoError(t, session.DB(conf.DatabaseName).C("perf_results").Find(nil).One(perfResult))
				assert.Equal(t, expectedResult.ID, perfResult.ID)
			},
		},
		{
			name: "WithAuthAndTLS",
			setUp: func(t *testing.T) *exec.Cmd {
				client, err := setupClient(ctx)
				require.NoError(t, err)

				caData, err := client.GetRootCertificate(ctx)
				require.NoError(t, err)
				userCertData, err := client.GetUserCertificate(ctx, os.Getenv("LDAP_USER"), os.Getenv("LDAP_PASSWORD"))
				require.NoError(t, err)
				userKeyData, err := client.GetUserCertificateKey(ctx, os.Getenv("LDAP_USER"), os.Getenv("LDAP_PASSWORD"))
				require.NoError(t, err)
				require.NoError(t, writeCerts(caData, caCert, userCertData, userCert, userKeyData, userKey))

				return createSendCommand(curatorPath, remoteAddress, caCert, userCert, userKey, false)
			},
			closer: func(t *testing.T) {
				client, err := setupClient(ctx)
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, os.Remove(caCert))
					assert.NoError(t, os.Remove(userCert))
					assert.NoError(t, os.Remove(userKey))
					_, err = client.RemovePerformanceResultById(ctx, expectedResult.ID)
					assert.NoError(t, err)
				}()

				perfResult, err := client.FindPerformanceResultById(ctx, expectedResult.ID)
				assert.NoError(t, err)
				require.NotNil(t, perfResult)
				assert.Equal(t, expectedResult.ID, *perfResult.Name)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.skip {
				t.Skip("test disabled")
			}
			if runtime.GOOS == "windows" {
				t.Skip("windows sucks")
			}

			cmd := test.setUp(t)
			assert.NoError(t, cmd.Run())
			test.closer(t)
		})
	}
}

func createSendCommand(curatorPath, address, caPath, userCertPath, userKeyPath string, insecure bool) *exec.Cmd {
	args := []string{
		"poplar",
		"send",
		"--service",
		address,
		"--path",
		filepath.Join("testdata", "mockTestResults.yaml"),
	}
	if insecure {
		args = append(args, "--insecure")
	} else {
		args = append(
			args,
			"--ca",
			caPath,
			"--cert",
			userCertPath,
			"--key",
			userKeyPath,
		)
	}

	return exec.Command(curatorPath, args...)
}

func setupClient(ctx context.Context) (*rest.Client, error) {
	opts := rest.ClientOptions{
		Host:     "https://cedar.mongodb.com",
		Port:     443,
		Prefix:   "rest",
		Username: os.Getenv("LDAP_USER"),
	}
	client, err := rest.NewClient(opts)
	if err != nil {
		return nil, err
	}
	apiKey, err := client.GetAuthKey(ctx, os.Getenv("LDAP_USER"), os.Getenv("LDAP_PASSWORD"))
	if err != nil {
		return nil, err
	}
	opts.ApiKey = apiKey

	client, err = rest.NewClientFromExisting(client.Client(), opts)
	return client, err
}

func writeCerts(ca, caPath, userCert, userCertPath, userKey, userKeyPath string) error {
	certs := map[string]string{
		caPath:       ca,
		userCertPath: userCert,
		userKeyPath:  userKey,
	}

	for filename, data := range certs {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(data)
		if err != nil {
			return err
		}
	}

	return nil
}
