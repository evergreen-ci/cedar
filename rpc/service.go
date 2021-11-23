package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/evergreen-ci/aviation"
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rpc/internal"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/recovery"
	"github.com/mongodb/grip/sometimes"
	"github.com/pkg/errors"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type WaitFunc func(context.Context)

type AuthConfig struct {
	TLS         bool
	UserAuth    bool
	SkipVerify  bool
	CAName      string
	ServiceName string
	Depot       certdepot.Depot
	UserManager gimlet.UserManager
}

func (c *AuthConfig) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.AddWhen(c.TLS && c.Depot == nil, errors.New("must specify a certificate depot"))
	catcher.AddWhen(c.TLS && c.CAName == "", errors.New("must specify a CA Name"))
	catcher.AddWhen(c.TLS && c.ServiceName == "", errors.New("must specify a service  Name"))
	catcher.AddWhen((c.TLS || c.UserAuth) && c.UserManager == nil, errors.New("must specify a user manager"))

	return catcher.Resolve()
}

func (c *AuthConfig) ResolveTLS() (*tls.Config, error) {
	// Load the certificates
	cert, err := certdepot.GetCertificate(c.Depot, c.ServiceName)
	if err != nil {
		return nil, errors.Wrap(err, "getting server certificate")
	}
	certPayload, err := cert.Export()
	if err != nil {
		return nil, errors.Wrap(err, "exporting server certificate")
	}
	key, err := certdepot.GetPrivateKey(c.Depot, c.ServiceName)
	if err != nil {
		return nil, errors.Wrap(err, "getting server certificate key")
	}
	keyPayload, err := key.ExportPrivate()
	if err != nil {
		return nil, errors.Wrap(err, "exporting server certificate key")
	}
	certificate, err := tls.X509KeyPair(certPayload, keyPayload)
	if err != nil {
		return nil, errors.Wrap(err, "loading server key pair")
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := certdepot.GetCertificate(c.Depot, c.CAName)
	if err != nil {
		return nil, errors.Wrap(err, "getting ca certificate")
	}
	caPayload, err := ca.Export()
	if err != nil {
		return nil, errors.Wrap(err, "exporting ca certificate")
	}

	// Append the client certificates from the CA
	if ok := certPool.AppendCertsFromPEM(caPayload); !ok {
		return nil, errors.New("appending client certs")
	}
	conf := &tls.Config{
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{certificate},
		ClientCAs:          certPool,
		InsecureSkipVerify: c.SkipVerify,
	}

	return conf, nil
}

func GetServer(env cedar.Environment, conf AuthConfig) (*grpc.Server, error) {
	logWhen := func() bool { return sometimes.Percent(10) }
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		aviation.MakeConditionalGripUnaryInterceptor(logging.MakeGrip(grip.GetSender()), logWhen),
	}
	streamInterceptors := []grpc.StreamServerInterceptor{
		aviation.MakeConditionalGripStreamInterceptor(logging.MakeGrip(grip.GetSender()), logWhen),
	}
	opts := []grpc.ServerOption{}

	if err := conf.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid auth config")
	}

	if conf.TLS {
		tlsConf, err := conf.ResolveTLS()
		if err != nil {
			return nil, errors.Wrap(err, "generating tls config")
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConf)))
		unaryInterceptors = append(unaryInterceptors, aviation.MakeCertificateUserValidationUnaryInterceptor(conf.UserManager))
		streamInterceptors = append(streamInterceptors, aviation.MakeCertificateUserValidationStreamInterceptor(conf.UserManager))
	}
	if conf.UserAuth {
		umConf := cedar.GetUserMiddlewareConfiguration()
		if err := umConf.Validate(); err != nil {
			return nil, errors.New("programmer error; invalid user manager configuration")
		}

		// The health check end point should not be protected.
		ignore := fmt.Sprintf("/%s/%s", internal.HealthServiceName(), "Check")

		unaryInterceptors = append(unaryInterceptors, aviation.MakeAuthenticationRequiredUnaryInterceptor(conf.UserManager, umConf, ignore))
		streamInterceptors = append(streamInterceptors, aviation.MakeAuthenticationRequiredStreamInterceptor(conf.UserManager, umConf, ignore))
	}

	opts = append(
		opts,
		grpc.UnaryInterceptor(aviation.ChainUnaryServer(unaryInterceptors...)),
		grpc.StreamInterceptor(aviation.ChainStreamServer(streamInterceptors...)),
		grpc.MaxRecvMsgSize(1e9),
	)

	srv := grpc.NewServer(opts...)

	services := map[string]internal.HealthCheckResponse_ServingStatus{}
	internal.AttachPerfService(env, srv)
	services[internal.PerfServiceName()] = internal.HealthCheckResponse_SERVING
	internal.AttachBuildloggerService(env, srv)
	services[internal.BuildloggerServiceName()] = internal.HealthCheckResponse_SERVING
	internal.AttachTestResultsService(env, srv)
	services[internal.TestResultsServiceName()] = internal.HealthCheckResponse_SERVING
	internal.AttachSystemMetricsService(env, srv)
	services[internal.SystemMetricsServiceName()] = internal.HealthCheckResponse_SERVING
	internal.AttachHealthService(services, srv)

	return srv, nil
}

func RunServer(ctx context.Context, srv *grpc.Server, addr string) (WaitFunc, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		defer recovery.LogStackTraceAndExit("running rpc service")
		grip.Warning(srv.Serve(lis))
	}()

	rpcWait := make(chan struct{})
	go func() {
		defer close(rpcWait)
		defer recovery.LogStackTraceAndContinue("waiting for the rpc service")
		<-ctx.Done()

		gracefulStop := make(chan struct{})
		go func() {
			defer close(gracefulStop)
			srv.GracefulStop()
		}()

		timer := time.NewTimer(2 * time.Minute)
		select {
		case <-gracefulStop:
		case <-timer.C:
			// In cases where it takes longer than 2 minutes to
			// to a graceful shutdown, we should just kill the grpc
			// server.
			srv.Stop()
		}

		grip.Info("rpc service terminated")
	}()

	return func(wctx context.Context) {
		select {
		case <-wctx.Done():
		case <-rpcWait:
		}
	}, nil
}
