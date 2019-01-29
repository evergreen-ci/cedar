package rpc

import (
	"context"
	"net"

	"github.com/evergreen-ci/aviation"
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rpc/internal"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func GetServer(env cedar.Environment, keyPath, certPath string) (*grpc.Server, error) {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(aviation.MakeGripUnaryInterceptor(logging.MakeGrip(grip.GetSender()))),
		grpc.StreamInterceptor(aviation.MakeGripStreamInterceptor(logging.MakeGrip(grip.GetSender()))),
	}

	if keyPath != "" || certPath != "" {
		creds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
		if err != nil {
			return nil, errors.Wrap(err, "problem reading certificates")
		}
		opts = append(opts, grpc.Creds(creds))
	} else {
		grip.Warning("certificates not defined, rpc service is starting without tls")
	}

	srv := grpc.NewServer(opts...)

	internal.AttachService(env, srv)

	return srv, nil
}

func RunServer(ctx context.Context, srv *grpc.Server, addr string) (func(), error) {
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

	return func() { <-rpcWait }, nil
}
