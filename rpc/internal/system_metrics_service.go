package internal

import (
	context "context"
	fmt "fmt"
	"io"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
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

// CreateSystemMetricsRecord creates a new system metrics record in the database.
func (s *systemMetricsService) CreateSystemMetricsRecord(ctx context.Context, data *SystemMetrics) (*SystemMetricsResponse, error) {
	conf := model.NewCedarConfig(s.env)
	if err := conf.Find(); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrap(err, "problem fetching cedar config"))
	}
	if conf.Bucket.SystemMetricsBucketType == "" {
		return nil, newRPCError(codes.Internal, errors.New("bucket type not specified"))
	}
	options := data.Artifact.Export()
	options.Type = conf.Bucket.SystemMetricsBucketType
	sm := model.CreateSystemMetrics(data.Info.Export(), options)

	sm.Setup(s.env)
	return &SystemMetricsResponse{Id: sm.ID}, newRPCError(codes.Internal, errors.Wrap(sm.SaveNew(ctx), "problem saving system metrics record"))
}

// AddSystemMetrics adds system metrics data to an existing bucket.
func (s *systemMetricsService) AddSystemMetrics(ctx context.Context, data *SystemMetricsData) (*SystemMetricsResponse, error) {
	systemMetrics := &model.SystemMetrics{ID: data.Id}
	systemMetrics.Setup(s.env)
	if err := systemMetrics.Find(ctx); err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message": "could not find system metrics",
			"op":      "adding system metrics",
			"context": "system metrics gRPC service",
		}))
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding systemMetrics record for '%s'", data.Id))
	}

	err := systemMetrics.Append(ctx, data.Type, data.Format.Export(), data.Data)
	grip.Error(message.WrapError(err, message.Fields{
		"message": "error appending system metrics",
		"op":      "adding system metrics",
		"context": "system metrics gRPC service",
	}))
	grip.InfoWhen(err == nil, message.Fields{
		"message": "successfully added system metrics",
		"op":      "adding system metrics",
		"context": "system metrics gRPC service",
	})
	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(err, "problem appending systemMetrics data for '%s'", data.Id))
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
		grip.Error(message.WrapError(err, message.Fields{
			"message": "error while receiving chunk",
			"op":      "streaming system metrics",
			"context": "system metrics gRPC service",
		}))
		if err == io.EOF {
			grip.Info(message.Fields{
				"message": "successfully finished streaming system metrics",
				"op":      "adding system metrics",
				"context": "system metrics gRPC service",
			})
			return stream.SendAndClose(&SystemMetricsResponse{Id: id})
		}
		if err != nil {
			return newRPCError(codes.Internal, errors.Wrapf(err, "error in stream for '%s'", id))
		}

		grip.Info(message.Fields{
			"message":    "successfully received chunk",
			"chunk_id":   chunk.Id,
			"chunk_type": chunk.Type,
			"op":         "streaming system metrics",
			"context":    "system metrics gRPC service",
		})
		if id == "" {
			id = chunk.Id
		} else if chunk.Id != id {
			return newRPCError(codes.Aborted, fmt.Errorf("systemMetrics ID %s in stream does not match reference %s, aborting", chunk.Id, id))
		}

		_, err = s.AddSystemMetrics(ctx, chunk)
		grip.Error(message.WrapError(err, message.Fields{
			"message":    "error while adding system metrics",
			"chunk_id":   chunk.Id,
			"chunk_type": chunk.Type,
			"op":         "streaming system metrics",
			"context":    "system metrics gRPC service",
		}))
		if err != nil {
			return newRPCError(codes.Internal, errors.Wrapf(err, "error in adding data for '%s'", id))
		}
		grip.Info(message.Fields{
			"message":    "successfully added system metrics to stream",
			"chunk_id":   chunk.Id,
			"chunk_type": chunk.Type,
			"op":         "adding system metrics",
			"context":    "system metrics gRPC service",
		})
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
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding system metrics record for '%s'", info.Id))
	}
	err := systemMetrics.Close(ctx, info.Success)
	grip.InfoWhen(err == nil, message.Fields{
		"message": "successfully finished streaming system metrics",
		"op":      "adding system metrics",
		"context": "system metrics gRPC service",
	})
	return &SystemMetricsResponse{Id: systemMetrics.ID},
		newRPCError(codes.Internal, errors.Wrapf(err, "problem closing system metrics record with id %s", systemMetrics.ID))
}
