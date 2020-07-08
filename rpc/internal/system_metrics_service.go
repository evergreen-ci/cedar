package internal

import (
	context "context"
	fmt "fmt"
	"io"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
)

type systemMetricsService struct {
	env cedar.Environment
}

// AttachSystemMetricsService attaches the systemMetrics service to the given gRPC
// server.
func AttachSystemMetricsService(env cedar.Environment, s *grpc.Server) {
	srv := &systemMetricsService{
		env: env,
	}
	RegisterCedarSystemMetricsServer(s, srv)
}

//
func (s *systemMetricsService) CreateSystemMetricRecord(ctx context.Context, data *SystemMetrics) (*SystemMetricsResponse, error) {
	sm := model.CreateSystemMetrics(data.Info.Export(), data.Artifact.Export())
	sm.Setup(s.env)
	return &SystemMetricsResponse{Id: sm.ID}, newRPCError(codes.Internal, errors.Wrap(sm.SaveNew(ctx), "problem saving system metrics record"))
}

// AddSystemMetrics adds system metrics data to an existing bucket.
func (s *systemMetricsService) AddSystemMetrics(ctx context.Context, data *SystemMetricsData) (*SystemMetricsResponse, error) {
	systemMetrics := &model.SystemMetrics{ID: data.Id}
	systemMetrics.Setup(s.env)
	if err := systemMetrics.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding systemMetrics record for '%s'", data.Id))
	}

	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(systemMetrics.Append(ctx, data.Data), "problem appending systemMetrics data for '%s'", data.Id))
}

// StreamSystemMetrics adds system metrics data via client-side streaming to an existing
// bucket.
func (s *systemMetricsService) StreamSystemMetrics(stream CedarSystemMetrics_StreamSystemMetricsServer) error {
	ctx := stream.Context()
	id := ""

	for {
		if err := ctx.Err(); err != nil {
			return newRPCError(codes.Aborted, errors.Wrapf(err, "stream aborted for id '%s'", id))
		}

		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&SystemMetricsResponse{Id: id})
		}
		if err != nil {
			return newRPCError(codes.Internal, errors.Wrapf(err, "error in stream for '%s'", id))
		}

		if id == "" {
			id = chunk.Id
		} else if chunk.Id != id {
			return newRPCError(codes.Aborted, fmt.Errorf("systemMetrics ID %s in stream does not match reference %s, aborting", chunk.Id, id))
		}

		_, err = s.AddSystemMetrics(ctx, chunk)
		if err != nil {
			return newRPCError(codes.Internal, errors.Wrapf(err, "error in adding data for '%s'", id))
		}
	}
}

func (s *systemMetricsService) CloseMetrics(ctx context.Context, info *SystemMetricsSeriesEnd) (*SystemMetricsResponse, error) {
	systemMetrics := &model.SystemMetrics{ID: info.Id}
	systemMetrics.Setup(s.env)
	if err := systemMetrics.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding system metrics record for '%s'", info.Id))
	}

	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(systemMetrics.Close(ctx), "problem closing system metrics record with id %s", systemMetrics.ID))
}
