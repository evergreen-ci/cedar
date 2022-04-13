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

	// UnimplementedCedarSystemMetricsServer must be embedded for forward
	// compatibility. See system_metrics_grpc.pb.go for more information.
	UnimplementedCedarSystemMetricsServer
}

// AttachSystemMetricsService attaches the systemMetrics service to the given gRPC
// server.
func AttachSystemMetricsService(env cedar.Environment, s *grpc.Server) {
	srv := &systemMetricsService{
		env: env,
	}
	RegisterCedarSystemMetricsServer(s, srv)
}

// SystemMetricsServiceName returns the grpc service identifier for this
// service.
func SystemMetricsServiceName() string {
	return CedarSystemMetrics_ServiceDesc.ServiceName
}

// CreateSystemMetricsRecord creates a new system metrics record.
func (s *systemMetricsService) CreateSystemMetricsRecord(ctx context.Context, data *SystemMetrics) (*SystemMetricsResponse, error) {
	conf := model.NewCedarConfig(s.env)
	if err := conf.Find(); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrap(err, "fetching Cedar config"))
	}
	if conf.Bucket.SystemMetricsBucketType == "" {
		return nil, newRPCError(codes.Internal, errors.New("bucket type not specified"))
	}
	options := data.Artifact.Export()
	options.Type = conf.Bucket.SystemMetricsBucketType
	sm := model.CreateSystemMetrics(data.Info.Export(), options)

	sm.Setup(s.env)
	return &SystemMetricsResponse{Id: sm.ID}, newRPCError(codes.Internal, errors.Wrap(sm.SaveNew(ctx), "saving system metrics record"))
}

// AddSystemMetrics adds system metrics data to an existing bucket.
func (s *systemMetricsService) AddSystemMetrics(ctx context.Context, data *SystemMetricsData) (*SystemMetricsResponse, error) {
	systemMetrics := &model.SystemMetrics{ID: data.Id}
	systemMetrics.Setup(s.env)
	if err := systemMetrics.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "finding system metrics record '%s'", data.Id))
	}

	err := systemMetrics.Append(ctx, data.Type, data.Format.Export(), data.Data)
	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(err, "appending system metrics data for record '%s'", data.Id))
}

// StreamSystemMetrics adds system metrics data via client-side streaming to an existing
// bucket.
func (s *systemMetricsService) StreamSystemMetrics(stream CedarSystemMetrics_StreamSystemMetricsServer) error {
	ctx := stream.Context()
	id := ""

	for {
		if err := ctx.Err(); err != nil {
			return newRPCError(codes.Aborted, errors.Wrapf(err, "stream aborted for system metrics record '%s'", id))
		}

		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&SystemMetricsResponse{Id: id})
		}
		if err != nil {
			return newRPCError(codes.Internal, errors.Wrapf(err, "receiving stream chunk for '%s'", id))
		}

		if id == "" {
			id = chunk.Id
		} else if chunk.Id != id {
			return newRPCError(codes.Aborted, fmt.Errorf("system metrics chunk '%s' in stream does not match reference chunk id '%s'", chunk.Id, id))
		}

		_, err = s.AddSystemMetrics(ctx, chunk)
		if err != nil {
			return newRPCError(codes.Internal, errors.Wrapf(err, "adding data for chunk '%s'", id))
		}
	}
}

// CloseMetrics "closes out" a system metrics record by setting the
// completed at timestamp and success boolean. This should be the last
// rpc call made on a system metrics record.
func (s *systemMetricsService) CloseMetrics(ctx context.Context, info *SystemMetricsSeriesEnd) (*SystemMetricsResponse, error) {
	systemMetrics := &model.SystemMetrics{ID: info.Id}
	systemMetrics.Setup(s.env)
	if err := systemMetrics.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "finding system metrics record '%s'", info.Id))
	}
	err := systemMetrics.Close(ctx, info.Success)
	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(err, "closing system metrics record '%s'", systemMetrics.ID))
}
