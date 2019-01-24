package internal

import (
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/ftdc/events"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

type perfService struct {
	env cedar.Environment
}

func AttachService(env cedar.Environment, s *grpc.Server) {
	srv := &perfService{
		env: env,
	}

	RegisterCedarPerformanceMetricsServer(s, srv)
}

func (srv *perfService) CreateMetricSeries(ctx context.Context, result *ResultData) (*MetricsResponse, error) {
	if result.Id == nil {
		return nil, errors.New("invalid data")
	}

	record, err := result.Export()
	if err != nil {
		return nil, errors.Wrap(err, "problem exporting result")
	}
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
		artifact, err := i.Export()
		if err != nil {
			return nil, errors.Wrap(err, "problem exporting artifacts")
		}
		record.Artifacts = append(record.Artifacts, *artifact)
	}

	record.Rollups.Setup(srv.env)
	if err := addRollups(record, result.Rollups); err != nil {
		return nil, errors.Wrap(err, "problem attaching rollups")
	}

	record.Setup(srv.env)
	if err := record.Save(); err != nil {
		return resp, errors.Wrapf(err, "problem saving document '%s'", record.ID)
	}
	resp.Success = true
	return resp, nil
}

func (srv *perfService) AttachArtifacts(ctx context.Context, artifactData *ArtifactData) (*MetricsResponse, error) {
	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.ID = artifactData.Id

	if err := record.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding record for '%v'", artifactData.Id)
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	for _, i := range artifactData.Artifacts {
		artifact, err := i.Export()
		if err != nil {
			return nil, errors.Wrap(err, "problem exporting artifacts")
		}
		record.Artifacts = append(record.Artifacts, *artifact)
	}

	record.Setup(srv.env)
	if err := record.Save(); err != nil {
		return resp, errors.Wrapf(err, "problem saving document '%s'", record.ID)
	}
	resp.Success = true
	return resp, nil
}

func (srv *perfService) AttachRollups(ctx context.Context, rollupData *RollupData) (*MetricsResponse, error) {
	record := &model.PerformanceResult{}
	record.Setup(srv.env)
	record.ID = rollupData.Id

	if err := record.Find(); err != nil {
		return nil, errors.Wrapf(err, "problem finding record for '%v'", rollupData.Id)
	}

	resp := &MetricsResponse{}
	resp.Id = record.ID

	record.Rollups.Setup(srv.env)
	if err := addRollups(record, rollupData.Rollups); err != nil {
		return nil, errors.Wrap(err, "problem attaching rollup data")
	}

	record.Setup(srv.env)
	if err := record.Save(); err != nil {
		return nil, errors.Wrapf(err, "problem saving document '%s'", record.ID)
	}

	resp.Success = true
	return resp, nil
}

func (srv *perfService) SendMetrics(stream CedarPerformanceMetrics_SendMetricsServer) error {
	// NOTE:
	//   - will probably require leaving this connection open for
	//     longer than we often do, which may lead to load
	//     balancer shenanigans

	ctx := stream.Context()
	catcher := grip.NewBasicCatcher()
	pipe := make(chan events.Performance)
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
				pipe <- *pp
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

func addRollups(record *model.PerformanceResult, rollups []*RollupValue) error {
	catcher := grip.NewBasicCatcher()

	for _, r := range rollups {
		catcher.Add(record.Rollups.Add(r.Name, int(r.Version), r.UserSubmitted, r.Type.Export(), r.Value))
	}

	record.Rollups.ProcessedAt = time.Now()
	record.Rollups.Count = len(record.Rollups.Stats)
	record.Rollups.Valid = !catcher.HasErrors()

	return catcher.Resolve()
}
