package data

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/pkg/errors"
)

// DBConnector is a struct that implements the Connector interface backed by
// the service layer of cedar.
type DBConnector struct {
	env cedar.Environment
}

// CreateNewDBConnector is the entry point for creating a new Connector backed
// by DBConnector.
func CreateNewDBConnector(env cedar.Environment) Connector {
	return &DBConnector{
		env: env,
	}
}

// MockConnector is a struct that implements the Connector interface backed by
// a mock cedar service layer.
type MockConnector struct {
	CachedPerformanceResults map[string]model.PerformanceResult
	ChildMap                 map[string][]string
	CachedLogs               map[string]model.Log
	CachedTestResults        map[string]model.TestResults
	CachedHistoricalTestData []model.AggregatedHistoricalTestData
	CachedSystemMetrics      map[string]model.SystemMetrics
	Users                    map[string]bool
	Bucket                   string

	env cedar.Environment
}

func (mc *MockConnector) getBucket(ctx context.Context, prefix string) (pail.Bucket, error) {
	bucketOpts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "creating bucket").Error(),
		}
	}

	return bucket, nil
}
