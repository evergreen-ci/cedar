package internal

import (
	"context"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
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
	return &TestResultsResponse{TestResultsRecordId: record.ID},
		newRPCError(codes.Internal, errors.Wrap(record.SaveNew(ctx), "problem saving test results record"))
}

// AddTestResults adds test results to an existing test results record.
func (s *testResultsService) AddTestResults(ctx context.Context, results *TestResults) (*TestResultsResponse, error) {
	return nil, nil
}

// StreamTestResults adds test results via client-side streaming to an existing
// test results record.
func (s *testResultsService) StreamTestResults(stream CedarTestResults_StreamTestResultsServer) error {
	return nil
}

// CloseTestResultsRecord "closes out" a test results record by setting the
// completed at timestamp. This should be the last rcp call made on a test
// results record.
func (s *testResultsService) CloseTestResultsRecord(ctx context.Context, info *TestResultsEndInfo) (*TestResultsResponse, error) {
	return nil, nil
}
