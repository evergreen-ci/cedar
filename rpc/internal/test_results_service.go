package internal

import (
	"context"
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type testResultsService struct {
	env cedar.Environment
}

// AttachTestResultsService attaches the test results service to the given gRPC
// server.
func AttachTestResultsService(env cedar.Environment, s *grpc.Server) {
	srv := &testResultsService{
		env: env,
	}
	RegisterCedarTestResultsServer(s, srv)
}

// TestResultsServiceName returns the grpc service identifier for this service.
func TestResultsServiceName() string {
	return _CedarTestResults_serviceDesc.ServiceName
}

// CreateTestResultsRecord creates a new test results record in the database.
func (s *testResultsService) CreateTestResultsRecord(ctx context.Context, info *TestResultsInfo) (*TestResultsResponse, error) {
	conf := model.NewCedarConfig(s.env)
	if err := conf.Find(); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrap(err, "problem fetching cedar config"))
	}
	if conf.Bucket.TestResultsBucketType == "" {
		return nil, newRPCError(codes.Internal, errors.New("bucket type not specified"))
	}

	exported, err := info.Export()
	if err != nil {
		return nil, newRPCError(codes.InvalidArgument, errors.Wrap(err, "exporting test results info"))
	}

	record := model.CreateTestResults(exported, conf.Bucket.TestResultsBucketType)
	record.Setup(s.env)
	if err := record.SaveNew(ctx); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrap(err, "problem saving test results record"))
	}
	return &TestResultsResponse{TestResultsRecordId: record.ID}, nil
}

// AddTestResults adds test results to an existing test results record.
func (s *testResultsService) AddTestResults(ctx context.Context, results *TestResults) (*TestResultsResponse, error) {
	record := &model.TestResults{ID: results.TestResultsRecordId}
	record.Setup(s.env)
	if err := record.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding test results record for '%s'", results.TestResultsRecordId))
	}

	exportedResults := make([]model.TestResult, len(results.Results))
	for i, result := range results.Results {
		exportedResult, err := result.Export()
		if err != nil {
			return nil, newRPCError(codes.InvalidArgument, errors.Wrapf(err, "problem exporting test result"))
		}
		exportedResult.TaskID = record.Info.TaskID
		exportedResult.Execution = record.Info.Execution
		exportedResults[i] = exportedResult
	}

	if err := record.Append(ctx, exportedResults); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem appending test results for '%s'", results.TestResultsRecordId))
	}

	if !record.Info.HistoricalDataDisabled {
		conf := model.NewCedarConfig(s.env)
		if err := conf.Find(); err != nil {
			grip.Error(message.WrapError(errors.Wrap(err, "finding cedar configuration"), message.Fields{
				"message":           "failed to update historical test data",
				"test_results_info": record.Info,
			}))
			// If we can't find the cedar configuration, we should
			// not update the historical test data for these
			// results.
			conf.Flags.DisableHistoricalTestData = true
		}

		if !conf.Flags.DisableHistoricalTestData {
			go s.updateHistoricalData(record, exportedResults)
		}
	}

	return &TestResultsResponse{TestResultsRecordId: record.ID}, nil
}

// StreamTestResults adds test results via client-side streaming to an existing
// test results record.
func (s *testResultsService) StreamTestResults(stream CedarTestResults_StreamTestResultsServer) error {
	var id string
	ctx := stream.Context()

	for {
		if err := ctx.Err(); err != nil {
			return newRPCError(codes.Aborted, err)
		}

		results, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&TestResultsResponse{TestResultsRecordId: id})
		}
		if err != nil {
			return err
		}

		if id == "" {
			id = results.TestResultsRecordId
		} else if results.TestResultsRecordId != id {
			return newRPCError(codes.Aborted, errors.New("test results record ID in stream does not match reference, aborting"))
		}

		_, err = s.AddTestResults(ctx, results)
		if err != nil {
			return err
		}
	}
}

// CloseTestResultsRecord "closes out" a test results record by setting the
// completed at timestamp. This should be the last rpc call made on a test
// results record.
func (s *testResultsService) CloseTestResultsRecord(ctx context.Context, info *TestResultsEndInfo) (*TestResultsResponse, error) {
	record := &model.TestResults{ID: info.TestResultsRecordId}
	record.Setup(s.env)
	if err := record.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, newRPCError(codes.NotFound, err)
		}
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem finding test results record for '%s'", info.TestResultsRecordId))
	}

	if err := record.Close(ctx); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem closing test results with id %s", record.ID))
	}
	return &TestResultsResponse{TestResultsRecordId: record.ID}, nil
}

func (s *testResultsService) updateHistoricalData(record *model.TestResults, results []model.TestResult) {
	defer func() {
		if err := recovery.HandlePanicWithError(recover(), nil, "historical test data update"); err != nil {
			grip.Error(message.WrapError(err, message.Fields{
				"message":           "failed to update historical test data",
				"test_results_info": record.Info,
			}))
		}
	}()

	taskName := record.Info.DisplayTaskName
	if taskName == "" {
		taskName = record.Info.TaskName
	}
	for _, res := range results {
		info := model.HistoricalTestDataInfo{
			Project:     record.Info.Project,
			Variant:     record.Info.Variant,
			TaskName:    taskName,
			TestName:    res.GetDisplayName(),
			RequestType: record.Info.RequestType,
			Date:        res.TestEndTime,
		}
		htd, err := model.CreateHistoricalTestData(info)
		if err != nil {
			grip.Error(message.WrapError(errors.Wrap(err, "creating historical test data"), message.Fields{
				"message":                   "failed to update historical test data",
				"test_results_info":         record.Info,
				"historical_test_data_info": info,
				"test_result":               res,
			}))
			continue
		}
		htd.Setup(s.env)

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		grip.Error(message.WrapError(htd.Update(ctx, res), message.Fields{
			"message":                   "failed to update historical test data",
			"test_results_info":         record.Info,
			"historical_test_data_info": info,
			"test_result":               res,
		}))
	}
}
