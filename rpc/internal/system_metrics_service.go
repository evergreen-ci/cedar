package internal

import (
	context "context"

	"github.com/evergreen-ci/cedar"
	"google.golang.org/grpc"
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
func (*systemMetricsService) CreateSystemMetricRecord(context.Context, *SystemMetrics) (*SystemMetricsResponse, error) {
	return nil, nil
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
