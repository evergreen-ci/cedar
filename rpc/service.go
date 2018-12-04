package rpc

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rpc/internal"
	grpc "google.golang.org/grpc"
)

func AttachService(env cedar.Environment, srv *grpc.Server) {
	internal.AttachService(env, srv)
}
