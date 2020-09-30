package internal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	fmt "fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/evergreen-ci/certdepot"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

func startPerfService(ctx context.Context, env cedar.Environment, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachPerfService(env, s)

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
		Username: os.Getenv("AUTH_USER"),
		ApiKey:   os.Getenv("AUTH_API_KEY"),
	}
	client, err := rest.NewClient(opts)
	if err != nil {
		return nil, err
	}

	client, err = rest.NewClientFromExisting(client.Client(), opts)
	return client, errors.Wrap(err, "problem getting client")
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
	assert.True(t, len(result.Rollups.Stats) > len(rollups), "%s", id)

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
			name: "TestValidData",
			data: &ResultData{
				Id: &ResultID{
					Project: "testProject",
					Version: "testVersion",
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
			port := getPort()
			var env cedar.Environment
			if !test.mockEnv {
				env = cedar.GetEnvironment()
			}
			defer func() {
				require.NoError(t, tearDownEnv(env, test.mockEnv))
			}()

			require.NoError(t, startPerfService(ctx, env, port))
			client, err := getGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			resp, err := client.CreateMetricSeries(ctx, test.data)
			require.Equal(t, test.expectedResp, resp)
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
				Id: &ResultID{},
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

			require.NoError(t, startPerfService(ctx, env, port))
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

func TestCuratorSend(t *testing.T) {
	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_INTEGRATION_TESTS")); skip {
		t.Skip("SKIP_INTEGRATION_TESTS is set, skipping integration test with Curator")
	}

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
		[]model.PerfRollupValue{},
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
				env := cedar.GetEnvironment()
				assert.NoError(t, startPerfService(ctx, env, 7000))

				return createSendCommand(curatorPath, "localhost:7000", "", "", "", true)
			},
			closer: func(t *testing.T) {
				env := cedar.GetEnvironment()
				defer func() {
					require.NoError(t, tearDownEnv(env, false))
				}()

				conf, session, err := cedar.GetSessionWithConfig(env)
				require.NoError(t, err)
				perfResult := &model.PerformanceResult{}
				assert.NoError(t, session.DB(conf.DatabaseName).C("perf_results").Find(struct{}{}).One(perfResult))
				assert.Equal(t, expectedResult.ID, perfResult.ID)
			},
		},
		{
			name: "WithAuthAndTLS",
			setup: func(t *testing.T) *exec.Cmd {
				caData, err := restClient.GetRootCertificate(ctx)
				require.NoError(t, err)
				userCertData, err := restClient.GetUserCertificate(ctx, os.Getenv("AUTH_USER"), os.Getenv("AUTH_PASSWORD"), os.Getenv("AUTH_API_KEY"))
				require.NoError(t, err)
				userKeyData, err := restClient.GetUserCertificateKey(ctx, os.Getenv("AUTH_USER"), os.Getenv("AUTH_PASSWORD"), os.Getenv("AUTH_API_KEY"))
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
	invalidUser := "invalid"
	pass := "password"
	certDB := "certDepot"
	collName := "depot"
	env := cedar.GetEnvironment()
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
		crt, err := restClient.GetUserCertificate(ctx, user, pass, "")
		assert.NoError(t, err)
		assert.NotEmpty(t, crt)
		u := &certdepot.User{}
		assert.NoError(t, session.DB(certDB).C(collName).FindId(user).One(u))
		assert.Equal(t, u.Cert, crt)
		assert.NotEmpty(t, u.PrivateKey)
		assert.NotEmpty(t, u.CertReq)

		key, err := restClient.GetUserCertificateKey(ctx, user, pass, "")
		assert.NoError(t, err)
		assert.Equal(t, u.PrivateKey, key)

		require.NoError(t, session.DB(certDB).C(collName).RemoveId(user))
	})
	t.Run("KeyGeneration", func(t *testing.T) {
		key, err := restClient.GetUserCertificateKey(ctx, user, pass, "")
		assert.NoError(t, err)
		u := &certdepot.User{}
		require.NoError(t, session.DB(certDB).C(collName).FindId(user).One(u))
		assert.NotEmpty(t, u.Cert)
		assert.Equal(t, u.PrivateKey, key)
		assert.NotEmpty(t, u.CertReq)

		crt, err := restClient.GetUserCertificate(ctx, user, pass, "")
		assert.NoError(t, err)
		assert.Equal(t, u.Cert, crt)
	})
	t.Run("ExpiredCertificate", func(t *testing.T) {
		depotOpts := &certdepot.MongoDBOptions{
			DatabaseName:   certDB,
			CollectionName: collName,
		}
		d, err := certdepot.NewMongoDBCertDepot(ctx, depotOpts)
		require.NoError(t, err)
		require.NoError(t, certdepot.DeleteCertificate(d, user))
		require.NoError(t, certdepot.DeleteCertificateSigningRequest(d, user))
		require.NoError(t, d.Delete(certdepot.PrivKeyTag(user)))
		opts := certdepot.CertificateOptions{
			CommonName: user,
			CA:         "test-root",
			Host:       user,
			Expires:    0,
		}
		require.NoError(t, opts.CertRequest(d))
		require.NoError(t, opts.Sign(d))

		oldCrt, err := certdepot.GetCertificate(d, user)
		require.NoError(t, err)
		oldCrtPayload, err := oldCrt.Export()
		require.NoError(t, err)

		crtPayload, err := restClient.GetUserCertificate(ctx, user, pass, "")
		assert.NoError(t, err)
		assert.NotEqual(t, oldCrtPayload, crtPayload)

		crt, err := certdepot.GetCertificate(d, user)
		require.NoError(t, err)
		rawCrt, err := crt.GetRawCertificate()
		require.NoError(t, err)

		assert.True(t, rawCrt.NotAfter.After(time.Now()))
	})
	t.Run("CertificateHandshakeValidUser", func(t *testing.T) {
		ca, err := restClient.GetRootCertificate(ctx)
		require.NoError(t, err)
		crt, err := restClient.GetUserCertificate(ctx, user, pass, "")
		require.NoError(t, err)
		key, err := restClient.GetUserCertificateKey(ctx, user, pass, "")
		require.NoError(t, err)
		grpcClient, err := getTLSGRPCClient(ctx, localAddress, []byte(ca), []byte(crt), []byte(key))
		require.NoError(t, err)

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
	t.Run("CertficiateHandshakeInvalidUser", func(t *testing.T) {
		ca, err := restClient.GetRootCertificate(ctx)
		require.NoError(t, err)
		crt, err := restClient.GetUserCertificate(ctx, invalidUser, pass, "")
		require.NoError(t, err)
		key, err := restClient.GetUserCertificateKey(ctx, invalidUser, pass, "")
		require.NoError(t, err)
		grpcClient, err := getTLSGRPCClient(ctx, localAddress, []byte(ca), []byte(crt), []byte(key))
		require.NoError(t, err)

		data := &ResultData{
			Id: &ResultID{
				Project: "testing",
			},
		}
		resp, err := grpcClient.CreateMetricSeries(ctx, data)
		require.Error(t, err)
		assert.Nil(t, resp)
	})

}
