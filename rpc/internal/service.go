package internal

import (
	"context"
	"io"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	grpc "google.golang.org/grpc"
)

type perfService struct {
	env sink.Environment
}

func AttachService(env sink.Environment, s *grpc.Server) {
	srv := &perfService{
		env: env,
	}

	RegisterSinkPerformanceMetricsServer(s, srv)

	return
}

func (srv *perfService) CreateMetricSeries(ctx context.Context, result *ResultData) (*MetricsResponse, error) {
	if result.Id == nil {
		return nil, errors.New("invalid data")
	}

	record := result.Export()
	record.Setup(srv.env)
	record.CreatedAt = time.Now()

	resp := &MetricsResponse{}
	resp.Id = record.ID

	if err := record.Save(); err != nil {
		return resp, errors.Wrap(err, "problem saving record")
	}
	resp.Success = true

	return resp, nil
}

func (srv *perfService) AttachResultData(ctx context.Context, result *ResultData) (*MetricsResponse, error) {
	if result.Id == nil {
		return nil, errors.New("invalid data")
	}

	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.Info = *result.Id.Export()
	record.ID = record.Info.ID()

	if err := record.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding record for '%v'", result.Id)
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	for _, i := range result.Artifacts {
		record.Source = append(record.Source, *i.Export())
	}

	if err := record.Save(); err != nil {
		return resp, errors.Wrapf(err, "problem saving document '%s'", record.ID)
	}

	resp.Success = true

	return resp, nil
}
func (srv *perfService) AttachAuxilaryData(ctx context.Context, result *ResultData) (*MetricsResponse, error) {
	if result.Id == nil {
		return nil, errors.New("invalid data")
	}

	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.Info = *result.Id.Export()
	record.ID = record.Info.ID()

	if err := record.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding record for '%v'", result.Id)
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	for _, i := range result.Artifacts {
		record.AuxilaryData = append(record.AuxilaryData, *i.Export())
	}

	if err := record.Save(); err != nil {
		return resp, errors.Wrapf(err, "problem saving document '%s'", record.ID)
	}

	resp.Success = true
	return resp, nil
}

func (srv *perfService) SendMetrics(stream SinkPerformanceMetrics_SendMetricsServer) error {
	// NOTE:
	//   - will probably require leaving this connection open for
	//     longer than we often do, which may lead to load
	//     balancer shenanigans

	ctx := stream.Context()
	catcher := grip.NewBasicCatcher()
	pipe := make(chan model.PerformancePoint)
	record := &model.PerformanceResult{}
	record.Setup(srv.env)

	go func() {
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
				if err = record.Find(); err != nil {
					catcher.Add(err)
					return
				}
			} else if point.Id != record.ID {
				catcher.Add(errors.New("metric point in stream does not match reference, aborting"))
				return
			}

			for _, event := range point.Event {
				pp, err := event.Export()
				if err != nil {
					catcher.Add(err)
					continue
				}
				pipe <- pp
			}
		}
	}()

	// TODO: this won't actually work: we need to get/create a
	// writer here, we'll do this
	catcher.Add(model.DumpPerformanceSeries(ctx, pipe, record, nil))

	return catcher.Resolve()
}

func (srv *perfService) CloseMetrics(ctx context.Context, end *MetricsSeriesEnd) (*MetricsResponse, error) {
	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.ID = end.GetId()
	if err := record.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding record for %s", record.ID)
	}

	record.CompletedAt = time.Now()
	if err := record.Save(); err != nil {
		return nil, errors.Wrapf(err, "problem saving record %s", record.ID)
	}

	return nil, nil
}
