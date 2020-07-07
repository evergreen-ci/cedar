package internal

import (
	context "context"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
func (s *systemMetricsService) CreateSystemMetricRecord(ctx context.Context, data SystemMetrics) (*SystemMetricsResponse, error) {
	sm := model.CreateSystemMetrics(data.Info.Export(), data.Artifact.Export())
	sm.Setup(s.env)
	return &SystemMetricsResponse{Id: sm.ID}, newRPCError(codes.Internal, errors.Wrap(sm.SaveNew(ctx), "problem saving system metrics record"))
}

//
func (*systemMetricsService) AddSystemMetrics(context.Context, *SystemMetricsData) (*SystemMetricsResponse, error) {
	return nil, nil
}

//
func (*systemMetricsService) StreamSystemMetrics(CedarSystemMetrics_StreamSystemMetricsServer) error {
	return nil
}

//
func (*systemMetricsService) CloseMetrics(context.Context, *SystemMetricsSeriesEnd) (*SystemMetricsResponse, error) {
	return nil, nil
}
