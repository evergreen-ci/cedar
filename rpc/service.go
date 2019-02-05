package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"

	"github.com/evergreen-ci/aviation"
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rpc/internal"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type WaitFunc func(context.Context)

type CertConfig struct {
	CA         string
	Cert       string
	Key        string
	SkipVerify bool
}

func (c *CertConfig) Validate() error {
	catcher := grip.NewBasicCatcher()

	if !util.FileExists(c.CA) {
		catcher.New("must specify a valid certificate authority path")
	}

	if !util.FileExists(c.Cert) {
		catcher.New("must specify a valid server certificate path")
	}

	if !util.FileExists(c.Key) {
		catcher.New("must specify a valid server certificate key path")
	}

	return catcher.Resolve()
}

func (c *CertConfig) Resolve() (*tls.Config, error) {
	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(c.Cert, c.Key)
	if err != nil {
		return nil, errors.Wrap(err, "could not load server key pair")
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(c.CA)
	if err != nil {
		return nil, errors.Wrap(err, "could not read ca certificate")
	}

	// Append the client certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("failed to append client certs")
	}
	conf := &tls.Config{
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{certificate},
		ClientCAs:          certPool,
		InsecureSkipVerify: c.SkipVerify,
	}

	return conf, nil
}

func GetServer(env cedar.Environment, conf CertConfig) (*grpc.Server, error) {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(aviation.MakeGripUnaryInterceptor(logging.MakeGrip(grip.GetSender()))),
		grpc.StreamInterceptor(aviation.MakeGripStreamInterceptor(logging.MakeGrip(grip.GetSender()))),
	}

	if err := conf.Validate(); err != nil {
		grip.Warning(errors.Wrap(err, "certificates not defined, rpc service is starting without tls"))
	} else {
		tlsConf, err := conf.Resolve()
		if err != nil {
			return nil, errors.Wrap(err, "problem generating tls config")
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConf)))
	}

	srv := grpc.NewServer(opts...)

	internal.AttachService(env, srv)

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
		srv.GracefulStop()
		grip.Info("rpc service terminated")
	}()

	return func(wctx context.Context) {
		select {
		case <-wctx.Done():
		case <-rpcWait:
		}
	}, nil
}
