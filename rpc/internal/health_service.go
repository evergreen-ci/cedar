package internal

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type healthService struct {
	services map[string]HealthCheckResponse_ServingStatus

	// UnimplementedHealthServer must be embedded for forward
	// compatibility. See health_grpc.pb.go for more information.
	UnimplementedHealthServer
}

// HealthServiceName returns the grpc service identifier for this service.
func HealthServiceName() string {
	return Health_ServiceDesc.ServiceName
}

// AttachHealthService attaches the health check service to the given gRPC
// server.
func AttachHealthService(services map[string]HealthCheckResponse_ServingStatus, s *grpc.Server) {
	srv := &healthService{services: services}
	RegisterHealthServer(s, srv)
}

func (s *healthService) Check(_ context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	status := HealthCheckResponse_SERVING
	if req.Service != "" {
		var ok bool
		status, ok = s.services[req.Service]
		if !ok {
			return nil, newRPCError(codes.NotFound, errors.Errorf("'%s' is not a registered service", req.Service))
		}
	}

	return &HealthCheckResponse{Status: status}, nil
}
