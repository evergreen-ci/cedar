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
func (s *systemMetricsService) CreateSystemMetricRecord(context.Context, *SystemMetrics) (*SystemMetricsResponse, error) {
	return nil, nil
}

// AddSystemMetrics adds system metrics data to an existing bucket.
func (s *systemMetricsService) AddSystemMetrics(ctx context.Context, chunk *SystemMetricsData) (*SystemMetricsResponse, error) {
	systemMetrics := &model.SystemMetrics{ID: chunk.Id}
	systemMetrics.Setup(s.env)
	if err := systemMetrics.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding systemMetrics record for '%s'", chunk.Id))
	}

	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(systemMetrics.Append(ctx, chunk.Data), "problem appending systemMetrics data for '%s'", chunk.Id))
}

// StreamSystemMetrics adds system metrics data via client-side streaming to an existing
// bucket.
func (s *systemMetricsService) StreamSystemMetrics(stream CedarSystemMetrics_StreamSystemMetricsServer) error {
	ctx := stream.Context()
	id := ""

	for {
		if err := ctx.Err(); err != nil {
			return newRPCError(codes.Aborted, err)
		}

		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&SystemMetricsResponse{Id: id})
		}
		if err != nil {
			return newRPCError(codes.Internal, err)
		}

		if id == "" {
			id = chunk.Id
		} else if chunk.Id != id {
			return newRPCError(codes.Aborted, fmt.Errorf("systemMetrics ID %s in stream does not match reference %s, aborting", chunk.Id, id))
		}

		_, err = s.AddSystemMetrics(ctx, chunk)
		if err != nil {
			return newRPCError(codes.Internal, err)
		}
	}
}

//
func (*systemMetricsService) CloseMetrics(context.Context, *SystemMetricsSeriesEnd) (*SystemMetricsResponse, error) {
	return nil, nil
}
