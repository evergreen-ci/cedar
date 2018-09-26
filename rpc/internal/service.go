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

func (srv *perfService) CreateMetricSeries(ctx context.Context, start *MetricsSeriesStart) (*MetricsResponse, error) {
	if start.Id == nil {
		return nil, errors.New("invalid data")
	}

	id := start.Id.Export()
	record := model.CreatePerformanceResult(*id, start.SourcePath)
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

func (srv *perfService) SendMetrics(stream SinkPerformanceMetrics_SendMetricsServer) error {
	// TODO:
	//   - get the ID on the first event. If subsequent events have
	//     different IDs then we probably have an error.
	//
	//   - instantiate a collector and a pail instance to upload
	//     the bytes of the collector for the chunk
	//
	//   - get all the events, add them to the collector
	//     when everything's done upload the thing to the pail and
	//     return the result.

	// NOTE:
	//   - will probably require leaving this connection open for
	//     longer than we often do, which may lead to load
	//     balancer shenanigans
	var ()
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&SendResponse{})
		}
		if err != nil {
			return errors.WithStack(err)
		}

		grip.Info(point)
	}
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
