package internal

import (
	"context"
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/evergreen-ci/cedar/units"
	"github.com/mongodb/amboy"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/ftdc/events"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type perfService struct {
	env cedar.Environment

	// UnimplementedCedarPerformanceMetricsServer must be embedded for
	// forward compatibility. See perf_grpc.pb.go for more information.
	UnimplementedCedarPerformanceMetricsServer
}

// AttachPerfService attaches the perf service to the given gRPC server.
func AttachPerfService(env cedar.Environment, s *grpc.Server) {
	srv := &perfService{
		env: env,
	}
	RegisterCedarPerformanceMetricsServer(s, srv)
}

// PerfServiceName returns the grpc service identifier for this service.
func PerfServiceName() string {
	return CedarPerformanceMetrics_ServiceDesc.ServiceName
}

// CreateMetricSeries creates a new performance result record.
func (srv *perfService) CreateMetricSeries(ctx context.Context, result *ResultData) (*MetricsResponse, error) {
	if result.Id == nil {
		return nil, newRPCError(codes.InvalidArgument, errors.New("invalid data"))
	}

	record := result.Export()
	record.Setup(srv.env)

	resp := &MetricsResponse{}
	resp.Id = record.ID

	if err := record.SaveNew(ctx); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrap(err, "saving record"))
	}

	grip.Info(message.Fields{
		"message":   "successfully added metric series",
		"task_id":   result.GetId().GetTaskId(),
		"execution": result.GetId().GetExecution(),
		"project":   result.GetId().GetProject(),
	})

	resp.Success = true
	return resp, nil
}

// AttachResultArtifacts attaches artifacts to an existing performance result.
func (srv *perfService) AttachArtifacts(ctx context.Context, artifactData *ArtifactData) (*MetricsResponse, error) {
	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.ID = artifactData.Id
	if err := record.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "finding perf result record '%s'", artifactData.Id))
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	for _, a := range artifactData.Artifacts {
		record.Artifacts = append(record.Artifacts, *a.Export())
	}

	record.Setup(srv.env)
	if err := record.AppendArtifacts(ctx, record.Artifacts); err != nil {
		return resp, newRPCError(codes.Internal, errors.Wrapf(err, "appending artifacts to perf result '%s'", record.ID))
	}

	resp.Success = true
	return resp, nil
}

// AttachRollups attaches rollups to an existing performance result.
func (srv *perfService) AttachRollups(ctx context.Context, rollupData *RollupData) (*MetricsResponse, error) {
	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.ID = rollupData.Id
	if err := record.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "finding perf result record for rollup '%s'", rollupData.Id))
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	record.Setup(srv.env)
	if err := record.MergeRollups(ctx, ExportRollupValues(rollupData.Rollups)); err != nil {
		return nil, newRPCError(codes.InvalidArgument, errors.Wrapf(err, "attaching rollup data for perf result '%s'", record.ID))
	}

	if record.Info.Mainline {
		processingJob := units.NewUpdateTimeSeriesJob(record.CreateUnanalyzedSeries())
		err := amboy.EnqueueUniqueJob(ctx, srv.env.GetRemoteQueue(), processingJob)

		if err != nil {
			return nil, newRPCError(codes.Internal, errors.Wrapf(err, "creating signal processing job for perf result '%s'", record.ID))
		}
	}

	resp.Success = true
	return resp, nil
}

// SendMetrics streams time series data for a performance result.
func (srv *perfService) SendMetrics(stream CedarPerformanceMetrics_SendMetricsServer) error {
	// NOTE:
	//   - will probably require leaving this connection open for
	//     longer than we often do, which may lead to load
	//     balancer shenanigans

	ctx := stream.Context()
	catcher := grip.NewBasicCatcher()
	pipe := make(chan events.Performance)
	count := 0
	record := &model.PerformanceResult{}
	record.Setup(srv.env)

	go func() {
		defer recovery.LogStackTraceAndContinue("processing metrics")
		defer close(pipe)
		for {
			if ctx.Err() != nil {
				return
			}

			point, err := stream.Recv()
			if err == io.EOF {
				catcher.Add(stream.SendAndClose(&SendResponse{}))
				return
			}
			if err != nil {
				catcher.Add(errors.WithStack(err))
				return
			}

			if record.IsNil() {
				record.ID = point.Id
				if err = record.Find(ctx); err != nil {
					catcher.Add(err)
					return
				}
			} else if point.Id != record.ID {
				catcher.New("metric point in stream does not match reference")
				return
			}

			for _, event := range point.Event {
				select {
				case <-ctx.Done():
				case pipe <- *event.Export():
					count++
				}
			}
		}
	}()

	// TODO: this won't actually work: we need to get/create a
	// writer here, we'll do this
	catcher.Add(model.DumpPerformanceSeries(ctx, pipe, record, nil))

	return catcher.Resolve()
}

// CloseLog "closes out" a performance result by setting the completed at
// timestamp. This should be the last rcp call made on a performance result.
func (srv *perfService) CloseMetrics(ctx context.Context, end *MetricsSeriesEnd) (*MetricsResponse, error) {
	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.ID = end.Id
	if err := record.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "finding perf result record for metric series '%s'", end.Id))
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	completedAt := time.Now()
	if end.CompletedAt != nil {
		completedAt = end.CompletedAt.AsTime()
	}

	record.Setup(srv.env)
	if err := record.Close(ctx, completedAt); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "closing perf result record '%s'", record.ID))
	}

	ftdcJobEnqueued, err := srv.addFTDCRollupsJob(ctx, record.ID, record.Artifacts)
	if err != nil {
		return nil, errors.Wrap(err, "creating FTDC rollups job")
	}

	// Only enqueue a new update time series job for mainline results if
	// there are user-submitted rollups AND no event data (FTDC artifact).
	// In the latter case, the FTDC rollups job will take care of
	// enqueueing the update time series job.
	if record.Info.Mainline && len(record.Rollups.Stats) > 0 && !ftdcJobEnqueued {
		processingJob := units.NewUpdateTimeSeriesJob(record.CreateUnanalyzedSeries())
		if err = amboy.EnqueueUniqueJob(ctx, srv.env.GetRemoteQueue(), processingJob); err != nil {
			return nil, newRPCError(codes.Internal, errors.Wrapf(err, "creating signal processing job for perf result '%s'", record.ID))
		}
	}

	resp.Success = true
	return resp, nil
}

// addFTDCRollupsJob enqueues a new FTDC rollups job if and only if there is an
// FTDC artifact present, specified by the `raw-events` artifact schema type. A
// boolean indicating whether or not a job was enqueued is returned.
func (srv *perfService) addFTDCRollupsJob(ctx context.Context, id string, artifacts []model.ArtifactInfo) (bool, error) {
	var hasEventData bool
	q := srv.env.GetRemoteQueue()
	num_artifacts := 0

	for _, artifact := range artifacts {
		if artifact.Schema != model.SchemaRawEvents {
			continue
		}

		if hasEventData {
			return true, newRPCError(codes.InvalidArgument, errors.New("cannot have more than one raw events artifact"))
		}
		hasEventData = true

		job, err := units.NewFTDCRollupsJob(id, &artifact, perf.DefaultRollupFactories(), false)
		if err != nil {
			return false, newRPCError(codes.InvalidArgument, errors.WithStack(err))
		}

		if err = q.Put(ctx, job); err != nil {
			return false, newRPCError(codes.Internal, errors.Wrap(err, "putting FTDC rollups job in the remote queue"))
		}
		num_artifacts += 1
	}
	grip.Info(message.Fields{
		"message":       "Just enqueued FTDC rollup jobs",
		"id":            id,
		"num_artifacts": num_artifacts,
	})

	return hasEventData, nil
}
