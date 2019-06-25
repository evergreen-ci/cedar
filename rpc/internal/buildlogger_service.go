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
		var err error
		log.CreatedAt, err = ptypes.Timestamp(data.CreatedAt)
		if err != nil {
			return nil, errors.Wrap(err, "problem converting timestamp value artifact")
		}
	}

	log.Setup(s.env)
	return &BuildloggerResponse{LogId: log.ID}, errors.Wrap(log.SaveNew(), "problem saving log record")
}

// AppendLogLines adds log lines to an existing buildlogger log.
func (s *buildloggerService) AppendLogLines(ctx context.Context, lines *LogLines) (*BuildloggerResponse, error) {
	// TODO: some type of size check? We should probably limit the size of
	// log lines.

	log := &model.Log{ID: lines.LogId}
	log.Setup(s.env)
	if err := log.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding log record for '%s'", lines.LogId)
	}

	exportedLines := []model.LogLine{}
	for _, line := range lines.Lines {
		exportedLine, err := line.Export()
		if err != nil {
			return nil, errors.Wrapf(err, "problem exporting log lines")
		}
		exportedLines = append(exportedLines, exportedLine)
	}

	log.Setup(s.env)
	return &BuildloggerResponse{LogId: log.ID}, errors.Wrapf(log.Append(exportedLines), "problem appending log lines for '%s'", lines.LogId)
}

func (s *buildloggerService) StreamLog(stream Buildlogger_StreamLogServer) error {
	return nil
}

// CloseLog "closes out" a buildlogger log by setting the completed at
// timestamp and the exit code. This should be the last rcp call made on a log.
func (s *buildloggerService) CloseLog(ctx context.Context, info *LogEndInfo) (*BuildloggerResponse, error) {
	log := &model.Log{ID: info.LogId}
	log.Setup(s.env)
	if err := log.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding log record for '%s'", info.LogId)
	}

	completedAt := time.Now()
	if info.CompletedAt != nil {
		var err error
		completedAt, err = ptypes.Timestamp(info.CompletedAt)
		if err != nil {
			return nil, errors.Wrap(err, "problem converting completed_at timestamp")
		}
	}

	log.Setup(s.env)
	return &BuildloggerResponse{LogId: log.ID}, errors.Wrapf(log.CloseLog(completedAt, int(info.ExitCode)), "problem closing log with id %s", log.ID)
}
