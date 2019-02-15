package internal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/certdepot"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/reporting"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	mgo "gopkg.in/mgo.v2"
)

const (
	localAddress  = "localhost:2289"
	remoteAddress = "cedar.mongodb.com:7070"
)

type MockEnv struct {
	queue   amboy.Queue
	session *mgo.Session
	conf    *cedar.Configuration
}

func (m *MockEnv) Configure(config *cedar.Configuration) error { m.conf = config; return nil }
func (m *MockEnv) GetConf() (*cedar.Configuration, error)      { return m.conf, nil }
func (m *MockEnv) SetLocalQueue(queue amboy.Queue) error       { m.queue = queue; return nil }
func (m *MockEnv) SetRemoteQueue(queue amboy.Queue) error      { m.queue = queue; return nil }
func (m *MockEnv) GetLocalQueue() (amboy.Queue, error)         { return m.queue, nil }
func (m *MockEnv) GetRemoteQueue() (amboy.Queue, error)        { return m.queue, nil }
func (m *MockEnv) GetSession() (*mgo.Session, error)           { return m.session, errors.New("mock err") }
func (m *MockEnv) Close(_ context.Context) error               { return nil }
func (m *MockEnv) RegisterCloser(_ string, _ cedar.CloserFunc) {}

func (m *MockEnv) GetRemoteReporter() (reporting.Reporter, error) {
	return nil, errors.New("not supported")
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

func getTLSGRPCClient(ctx context.Context, address string, ca, crt, key []byte) (CedarPerformanceMetricsClient, error) {
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(ca) {
		return nil, errors.New("credentials: failed to append certificates")
	}
	keyPair, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return nil, errors.Wrap(err, "problem reading client cert")
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		RootCAs:      cp,
	}

	return getGRPCClient(ctx, address, []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConf))})
}

func setupAuthRestClient(ctx context.Context, host string, port int) (*rest.Client, error) {
	opts := rest.ClientOptions{
		Host:     host,
		Port:     port,
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

func createEnv(mock bool) (cedar.Environment, error) {
	if mock {
		return &MockEnv{}, nil
	}
	env := cedar.GetEnvironment()
	err := env.Configure(&cedar.Configuration{
		MongoDBURI:         "mongodb://localhost:27017",
		DatabaseName:       "grpc_test",
		SocketTimeout:      time.Hour,
		NumWorkers:         2,
		DisableRemoteQueue: true,
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
			client, err := getGRPCClient(ctx, localAddress, []grpc.DialOption{grpc.WithInsecure()})
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
			client, err := getGRPCClient(ctx, localAddress, []grpc.DialOption{grpc.WithInsecure()})
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
	restClient, err := setupAuthRestClient(ctx, "https://cedar.mongodb.com", 443)
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		skip   bool
		setup  func(t *testing.T) *exec.Cmd
		closer func(t *testing.T)
	}{
		{
			name: "WithoutAuthOrTLS",
			setup: func(t *testing.T) *exec.Cmd {
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
			setup: func(t *testing.T) *exec.Cmd {
				caData, err := restClient.GetRootCertificate(ctx)
				require.NoError(t, err)
				userCertData, err := restClient.GetUserCertificate(ctx, os.Getenv("LDAP_USER"), os.Getenv("LDAP_PASSWORD"))
				require.NoError(t, err)
				userKeyData, err := restClient.GetUserCertificateKey(ctx, os.Getenv("LDAP_USER"), os.Getenv("LDAP_PASSWORD"))
				require.NoError(t, err)
				require.NoError(t, writeCerts(caData, caCert, userCertData, userCert, userKeyData, userKey))

				return createSendCommand(curatorPath, remoteAddress, caCert, userCert, userKey, false)
			},
			closer: func(t *testing.T) {
				defer func() {
					assert.NoError(t, os.Remove(caCert))
					assert.NoError(t, os.Remove(userCert))
					assert.NoError(t, os.Remove(userKey))
					_, err := restClient.RemovePerformanceResultById(ctx, expectedResult.ID)
					assert.NoError(t, err)
				}()

				perfResult, err := restClient.FindPerformanceResultById(ctx, expectedResult.ID)
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

			cmd := test.setup(t)
			assert.NoError(t, cmd.Run())
			test.closer(t)
		})
	}
}

func TestCertificateGeneration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows really sucks")
	}

	user := "evergreen"
	pass := "password"
	certDB := "certDepot"
	collName := "depot"
	env, envErr := createEnv(false)
	require.NoError(t, envErr)
	defer func() {
		assert.NoError(t, tearDownEnv(env, false))
	}()

	conf, session, envErr := cedar.GetSessionWithConfig(env)
	require.NoError(t, envErr)
	defer func() {
		assert.NoError(t, session.DB(certDB).DropDatabase())
	}()

	cedarExecutable, pathErr := filepath.Abs(filepath.Join("..", "..", "build", "cedar"))
	require.NoError(t, pathErr)
	_, statErr := os.Stat(cedarExecutable)
	require.NoError(t, statErr)

	cmd := exec.Command(
		cedarExecutable,
		"admin",
		"conf",
		"load",
		"--file",
		filepath.Join("testdata", "cedarconf.yaml"),
		"--dbName",
		conf.DatabaseName,
	)
	require.NoError(t, cmd.Run())

	cmd = exec.Command(cedarExecutable, "service", "--dbName", conf.DatabaseName)
	grip.Noticeln(cedarExecutable, "service", "--dbName", conf.DatabaseName)
	grip.SetName("curator.rpc.test")
	writer := send.NewWriterSender(grip.GetSender())
	defer func() { require.NoError(t, writer.Close()) }()
	cmd.Stderr = writer
	cmd.Stdout = writer
	require.NoError(t, cmd.Start())
	go func() { grip.Warning(cmd.Wait()) }()
	defer func() { require.NoError(t, cmd.Process.Kill()) }()
	time.Sleep(time.Second * 5)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	opts := rest.ClientOptions{
		Host:   "http://localhost",
		Port:   3000,
		Prefix: "rest",
	}
	restClient, clientErr := rest.NewClient(opts)
	require.NoError(t, clientErr)

	t.Run("RootAndServerGeneration", func(t *testing.T) {
		rootcrt, err := restClient.GetRootCertificate(ctx)
		require.NoError(t, err)
		u := &certdepot.User{}
		assert.NoError(t, session.DB(certDB).C(collName).FindId("test-root").One(u))
		assert.Equal(t, rootcrt, u.Cert)
		assert.NotEmpty(t, u.PrivateKey)
		assert.NotEmpty(t, u.CertRevocList)

		u = &certdepot.User{}
		assert.NoError(t, session.DB(certDB).C(collName).FindId("localhost").One(u))
		assert.NotEmpty(t, u.Cert)
		assert.NotEmpty(t, u.PrivateKey)
		assert.NotEmpty(t, u.CertReq)
	})
	t.Run("CertificateGeneration", func(t *testing.T) {
		crt, err := restClient.GetUserCertificate(ctx, user, pass)
		assert.NoError(t, err)
		assert.NotEmpty(t, crt)
		u := &certdepot.User{}
		assert.NoError(t, session.DB(certDB).C(collName).FindId(user).One(u))
		assert.Equal(t, u.Cert, crt)
		assert.NotEmpty(t, u.PrivateKey)
		assert.NotEmpty(t, u.CertReq)

		key, err := restClient.GetUserCertificateKey(ctx, user, pass)
		assert.NoError(t, err)
		assert.Equal(t, u.PrivateKey, key)

		require.NoError(t, session.DB(certDB).C(collName).RemoveId(user))
	})
	t.Run("KeyGeneration", func(t *testing.T) {
		key, err := restClient.GetUserCertificateKey(ctx, user, pass)
		assert.NoError(t, err)
		u := &certdepot.User{}
		require.NoError(t, session.DB(certDB).C(collName).FindId(user).One(u))
		assert.NotEmpty(t, u.Cert)
		assert.Equal(t, u.PrivateKey, key)
		assert.NotEmpty(t, u.CertReq)

		crt, err := restClient.GetUserCertificate(ctx, user, pass)
		assert.NoError(t, err)
		assert.Equal(t, u.Cert, crt)
	})

	ca, err := restClient.GetRootCertificate(ctx)
	require.NoError(t, err)
	crt, err := restClient.GetUserCertificate(ctx, user, pass)
	require.NoError(t, err)
	key, err := restClient.GetUserCertificateKey(ctx, user, pass)
	require.NoError(t, err)
	grpcClient, err := getTLSGRPCClient(ctx, localAddress, []byte(ca), []byte(crt), []byte(key))
	require.NoError(t, err)
	t.Run("CertificateHandshake", func(t *testing.T) {
		data := &ResultData{
			Id: &ResultID{
				Project: "testing",
			},
		}
		resp, err := grpcClient.CreateMetricSeries(ctx, data)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.Id)
	})
}
