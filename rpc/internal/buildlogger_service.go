package internal

import (
	"context"
	"io"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type buildloggerService struct {
	env cedar.Environment
}

// AttachBuildloggerService attaches the buildlogger service to the given gRPC
// server.
func AttachBuildloggerService(env cedar.Environment, s *grpc.Server) {
	srv := &buildloggerService{
		env: env,
	}
	RegisterBuildloggerServer(s, srv)
}

// CreateLog creates a new buildlogger log record in the database.
func (s *buildloggerService) CreateLog(ctx context.Context, data *LogData) (*BuildloggerResponse, error) {
	log := model.CreateLog(data.Info.Export(), data.Storage.Export())
	log.Setup(s.env)
	return &BuildloggerResponse{LogId: log.ID}, newRPCError(codes.Internal, errors.Wrap(log.SaveNew(ctx), "problem saving log record"))
}

// AppendLogLines adds log lines to an existing buildlogger log.
func (s *buildloggerService) AppendLogLines(ctx context.Context, lines *LogLines) (*BuildloggerResponse, error) {
	// TODO: some type of size check? We should probably limit the size of
	// log lines.

	log := &model.Log{ID: lines.LogId}
	log.Setup(s.env)
	if err := log.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding log record for '%s'", lines.LogId))
	}

	exportedLines := []model.LogLine{}
	for _, line := range lines.Lines {
		exportedLine, err := line.Export()
		if err != nil {
			return nil, newRPCError(codes.InvalidArgument, errors.Wrapf(err, "problem exporting log lines"))
		}
		exportedLines = append(exportedLines, exportedLine)
	}

	return &BuildloggerResponse{LogId: log.ID},
		newRPCError(codes.Internal, errors.Wrapf(log.Append(ctx, exportedLines), "problem appending log lines for '%s'", lines.LogId))
}

// StreamLogLines adds log lines via client-side streaming to an existing
// buildlogger log.
func (s *buildloggerService) StreamLogLines(stream Buildlogger_StreamLogLinesServer) error {
	ctx := stream.Context()
	id := ""

	for {
		if err := ctx.Err(); err != nil {
			return newRPCError(codes.Aborted, err)
		}

		lines, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&BuildloggerResponse{LogId: id})
		}
		if err != nil {
			return err
		}

		if id == "" {
			id = lines.LogId
		} else if lines.LogId != id {
			return newRPCError(codes.Aborted, errors.New("log ID in stream does not match reference, aborting"))
		}

		_, err = s.AppendLogLines(ctx, lines)
		if err != nil {
			return err
		}
	}
}

// CloseLog "closes out" a buildlogger log by setting the completed at
// timestamp and the exit code. This should be the last rcp call made on a log.
func (s *buildloggerService) CloseLog(ctx context.Context, info *LogEndInfo) (*BuildloggerResponse, error) {
	log := &model.Log{ID: info.LogId}
	log.Setup(s.env)
	if err := log.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding log record for '%s'", info.LogId))
	}

	return &BuildloggerResponse{LogId: log.ID},
		newRPCError(codes.Internal, errors.Wrapf(log.Close(ctx, int(info.ExitCode)), "problem closing log with id %s", log.ID))
}
