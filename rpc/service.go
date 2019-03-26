package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"

	"github.com/evergreen-ci/aviation"
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rpc/internal"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type WaitFunc func(context.Context)

type CertConfig struct {
	TLS         bool
	SkipVerify  bool
	CAName      string
	ServiceName string
	Depot       depot.Depot
	UserManager gimlet.UserManager
}

func (c *CertConfig) Validate() error {
	if c.Depot == nil {
		return errors.New("must specify a certificate depot!")
	}
	if c.CAName == "" {
		return errors.New("must specify a CA name!")
	}
	if c.ServiceName == "" {
		return errors.New("must specify a server name!")
	}

	return nil
}

func (c *CertConfig) Resolve() (*tls.Config, error) {
	// Load the certificates
	cert, err := depot.GetCertificate(c.Depot, c.ServiceName)
	if err != nil {
		return nil, errors.Wrap(err, "problem getting server certificate")
	}
	certPayload, err := cert.Export()
	if err != nil {
		return nil, errors.Wrap(err, "problem exporting server certificate")
	}
	key, err := depot.GetPrivateKey(c.Depot, c.ServiceName)
	if err != nil {
		return nil, errors.Wrap(err, "problem getting server certificate key")
	}
	keyPayload, err := key.ExportPrivate()
	if err != nil {
		return nil, errors.Wrap(err, "problem exporting server certificate key")
	}
	certificate, err := tls.X509KeyPair(certPayload, keyPayload)
	if err != nil {
		return nil, errors.Wrap(err, "problem loading server key pair")
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := depot.GetCertificate(c.Depot, c.CAName)
	if err != nil {
		return nil, errors.Wrap(err, "problem getting ca certificate")
	}
	caPayload, err := ca.Export()
	if err != nil {
		return nil, errors.Wrap(err, "problem exporting ca certificate")
	}

	// Append the client certificates from the CA
	if ok := certPool.AppendCertsFromPEM(caPayload); !ok {
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
	unaryInterceptors := []grpc.UnaryServerInterceptor{aviation.MakeGripUnaryInterceptor(logging.MakeGrip(grip.GetSender()))}
	streamInterceptors := []grpc.StreamServerInterceptor{aviation.MakeGripStreamInterceptor(logging.MakeGrip(grip.GetSender()))}
	opts := []grpc.ServerOption{}

	if err := conf.Validate(); conf.TLS && err != nil {
		return nil, errors.Wrap(err, "invalid tls config")
	} else if conf.TLS {
		tlsConf, err := conf.Resolve()
		if err != nil {
			return nil, errors.Wrap(err, "problem generating tls config")
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConf)))
		unaryInterceptors = append(unaryInterceptors, aviation.MakeCertificateUserValidationUnaryInterceptor(conf.UserManager))
		streamInterceptors = append(streamInterceptors, aviation.MakeCertificateUserValidationStreamInterceptor(conf.UserManager))
	}

	opts = append(
		opts,
		grpc.UnaryInterceptor(aviation.ChainUnaryServer(unaryInterceptors...)),
		grpc.StreamInterceptor(aviation.ChainStreamServer(streamInterceptors...)),
	)

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
