package data

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/units"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// FindTestResultsByTaskId queries the database to find the test
// result with the given id.
func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, id string) (*dataModel.APITestResult, error) {
	result := model.TestResult{}
	result.Setup(dbc.env)
	result.ID = id

	if err := result.Find(ctx); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test result with id '%s' not found", id),
		}
	}

	apiResult := dataModel.APITestResult{}
	err := apiResult.Import(result)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("corrupt data"),
		}
	}
	return &apiResult, nil
}