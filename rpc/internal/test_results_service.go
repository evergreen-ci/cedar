package internal

import (
	"context"
	"io"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/anser/db"
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

// CreateTestResultsRecord creates a new test results record in the database.
func (s *testResultsService) CreateTestResultsRecord(ctx context.Context, info *TestResultsInfo) (*TestResultsResponse, error) {
	conf := model.NewCedarConfig(s.env)
	if err := conf.Find(); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrap(err, "problem fetching cedar config"))
	}
	if conf.Bucket.TestResultsBucketType == "" {
		conf.Bucket.TestResultsBucketType = model.PailLocal
	}

	record := model.CreateTestResults(info.Export(), conf.Bucket.TestResultsBucketType)
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
		exportedResults[i] = exportedResult
	}

	if err := record.Append(ctx, exportedResults); err != nil {
		return nil, newRPCError(codes.Internal, errors.Wrapf(err, "problem appending test results for '%s'", results.TestResultsRecordId))
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
// completed at timestamp. This should be the last rcp call made on a test
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
