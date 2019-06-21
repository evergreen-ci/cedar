package internal

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
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

// CreateLog creates a new buildlogger log record in the database.
func (s *buildloggerService) CreateLog(ctx context.Context, data *LogData) (*BuildloggerResponse, error) {
	log := model.CreateLog(data.Info.Export(), data.Storage.Export())
	log.CreatedAt = time.Now()
	if data.CreatedAt != nil {
		ts, err := ptypes.Timestamp(data.CreatedAt)
		if err != nil {
			return nil, errors.Wrap(err, "problem converting timestamp value artifact")
		}
		log.CreatedAt = ts
	}
	log.Setup(s.env)

	resp := &BuildloggerResponse{}
	resp.LogId = log.ID

	if err := log.SaveNew(); err != nil {
		return nil, errors.Wrap(err, "problem saving log record")
	}

	return resp, nil
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
