package internal

import (
	"context"

	"github.com/evergreen-ci/cedar"
	"google.golang.org/grpc"
)

type buildloggerService struct {
	env cedar.Environment
}

func AttachBuildloggerService(env cedar.Environment, s *grpc.Server) {
	srv := &buildloggerService{
		env: env,
	}
	RegisterBuildloggerServer(s, srv)
}

func (s *buildloggerService) CreateLog(ctx context.Context, info *LogInfo) (*BuildloggerResponse, error) {
	return nil, nil
}

func (s *buildloggerService) AppendLogLines(ctx context.Context, info *LogLines) (*BuildloggerResponse, error) {
	return nil, nil
}

func (s *buildloggerService) StreamLog(stream Buildlogger_StreamLogServer) error {
	return nil
}

func (s *buildloggerService) CloseLog(ctx context.Context, info *LogEndInfo) (*BuildloggerResponse, error) {
	return nil, nil
}
