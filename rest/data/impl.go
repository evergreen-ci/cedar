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
// the service layer of Cedar.
type DBConnector struct {
	env     cedar.Environment
	baseURL string // Cache the base URL since it will likely not change.
}

// CreateNewDBConnector is the entry point for creating a new Connector backed
// by DBConnector.
func CreateNewDBConnector(env cedar.Environment, baseURL string) Connector {
	return &DBConnector{
		env:     env,
		baseURL: baseURL,
	}
}

func (dbc *DBConnector) GetBaseURL() string { return dbc.baseURL }

// MockConnector is a struct that implements the Connector interface backed by
// a mock Cedar service layer.
type MockConnector struct {
	ChildMap   map[string][]string
	CachedLogs map[string]model.Log
	Users      map[string]bool
	Bucket     string

	env cedar.Environment
}

func (mc *MockConnector) GetBaseURL() string { return "https://mock.com" }

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
