package rest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

const (
	// Requester API values.
	htdAPIRequesterMainline = "mainline"
	htdAPIRequesterPatch    = "patch"
	htdAPIRequesterTrigger  = "trigger"
	htdAPIRequesterGitTag   = "git_tag"
	htdAPIRequesterAdhoc    = "adhoc"

	// Sort API values.
	htdAPISortEarliest = "earliest"
	htdAPISortLatest   = "latest"

	// GroupBy API values for tests.
	htdAPITestGroupByDistro  = "test_task_variant_distro"
	htdAPITestGroupByVariant = "test_task_variant"
	htdAPITestGroupByTask    = "test_task"
	htdAPITestGroupByTest    = "test"

	// GroupBy API values for tasks.
	htdAPITaskGroupByDistro  = "task_variant_distro"
	htdAPITaskGroupByVariant = "task_variant"
	htdAPITaskGroupByTask    = "task"

	// API Limits.
	htdAPIMaxGroupNumDays = 26 * 7 // 26 weeks which is the maximum amount of data available
	htdAPIMaxNumTests     = 50
	htdAPIMaxNumTasks     = 50
	htdAPIMaxLimit        = 1000

	// Format used to encode dates in the API.
	htdAPIDateFormat = "2006-01-02"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /historical_test_data/{project_id}

type historicalTestDataHandler struct {
	sc data.Connector
	htdFilterHandler
}

func (h *historicalTestDataHandler) Factory() gimlet.RouteHandler {
	return &historicalTestDataHandler{sc: h.sc}
}

func (h *historicalTestDataHandler) Parse(ctx context.Context, r *http.Request) error {
	h.filter = model.HistoricalTestDataFilter{Project: gimlet.GetVars(r)["project_id"]}

	err := h.parse(r.URL.Query())
	if err != nil {
		return errors.Wrap(err, "invalid query parameters")
	}

	err = h.filter.Validate()
	if err != nil {
		return gimlet.ErrorResponse{
			Message:    err.Error(),
			StatusCode: http.StatusBadRequest,
		}
	}

	return nil
}

func (h *historicalTestDataHandler) Run(ctx context.Context) gimlet.Responder {
	data, err := h.sc.GetHistoricalTestData(ctx, h.filter)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "problem to fetching historical test data"))
	}

	resp := gimlet.NewJSONResponse(nil)
	requestLimit := h.filter.Limit - 1
	lastIndex := len(data)
	if len(data) > requestLimit {
		lastIndex = requestLimit

		err = resp.SetPages(&gimlet.ResponsePages{
			Next: &gimlet.Page{
				Relation:        "next",
				LimitQueryParam: "limit",
				KeyQueryParam:   "start_at",
				BaseURL:         baseURL,
				Key:             data[requestLimit].StartAtKey(),
				Limit:           requestLimit,
			},
		})
		if err != nil {
			return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err,
				"problem paginating response"))
		}
	}
	if err = resp.AddData(data[:lastIndex]); err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "problem creating response"))
	}

	return resp
}

func makeGetHistoricalTestData(sc data.Connector) gimlet.RouteHandler {
	return &historicalTestDataHandler{sc: sc}
}

// htdFilterHandler handles parsing the url query and populating the
// HistoricalTestDataFilter for the request.
type htdFilterHandler struct {
	filter model.HistoricalTestDataFilter
}

// parse parses the query parameter values and fills the struct filter field.
func (h *htdFilterHandler) parse(vals url.Values) error {
	var err error

	h.filter.Requesters, err = h.readRequesters(h.readStringList(vals["requesters"]))
	if err != nil {
		return gimlet.ErrorResponse{
			Message:    "invalid requesters value",
			StatusCode: http.StatusBadRequest,
		}
	}

	h.filter.Variants = h.readStringList(vals["variants"])

	h.filter.GroupNumDays, err = h.readInt(vals.Get("group_num_days"), 1, htdAPIMaxGroupNumDays, 1)
	if err != nil {
		return gimlet.ErrorResponse{
			Message:    "invalid group_num_days value",
			StatusCode: http.StatusBadRequest,
		}
	}

	h.filter.StartAt, err = h.readStartAt(vals.Get("start_at"))
	if err != nil {
		return err
	}

	h.filter.GroupBy, err = h.readGroupBy(vals.Get("group_by"))
	if err != nil {
		return err
	}

	h.filter.Limit, err = h.readInt(vals.Get("limit"), 1, htdAPIMaxLimit, htdAPIMaxLimit)
	if err != nil {
		return gimlet.ErrorResponse{
			Message:    "invalid limit value",
			StatusCode: http.StatusBadRequest,
		}
	}
	// Add 1 for pagination.
	h.filter.Limit++

	h.filter.Tasks = h.readStringList(vals["tasks"])
	if len(h.filter.Tasks) > htdAPIMaxNumTasks {
		return gimlet.ErrorResponse{
			Message:    "too many tasks values",
			StatusCode: http.StatusBadRequest,
		}
	}

	beforeDate := vals.Get("before_date")
	if beforeDate == "" {
		return gimlet.ErrorResponse{
			Message:    "missing before_date parameter",
			StatusCode: http.StatusBadRequest,
		}
	}
	h.filter.BeforeDate, err = time.ParseInLocation(htdAPIDateFormat, beforeDate, time.UTC)
	if err != nil {
		return gimlet.ErrorResponse{
			Message:    "invalid before_date value",
			StatusCode: http.StatusBadRequest,
		}
	}

	afterDate := vals.Get("after_date")
	if afterDate == "" {
		return gimlet.ErrorResponse{
			Message:    "missing after_date parameter",
			StatusCode: http.StatusBadRequest,
		}
	}
	h.filter.AfterDate, err = time.ParseInLocation(htdAPIDateFormat, afterDate, time.UTC)
	if err != nil {
		return gimlet.ErrorResponse{
			Message:    "invalid after_date value",
			StatusCode: http.StatusBadRequest,
		}
	}

	h.filter.Tests = h.readStringList(vals["tests"])
	if len(h.filter.Tests) > htdAPIMaxNumTests {
		return gimlet.ErrorResponse{
			Message:    "too many tests values",
			StatusCode: http.StatusBadRequest,
		}
	}

	h.filter.Sort, err = h.readSort(vals.Get("sort"))
	if err != nil {
		return err
	}

	return err
}

// readRequesters parses requesters parameter values and translates them into a
// list of internal Evergreen requester names.
func (h *htdFilterHandler) readRequesters(requesters []string) ([]string, error) {
	if len(requesters) == 0 {
		requesters = []string{htdAPIRequesterMainline}
	}

	requesterValues := []string{}
	for _, requester := range requesters {
		switch requester {
		case htdAPIRequesterMainline:
			requesterValues = append(requesterValues, cedar.RepotrackerVersionRequester)
		case htdAPIRequesterPatch:
			requesterValues = append(requesterValues, cedar.PatchRequesters...)
		case htdAPIRequesterTrigger:
			requesterValues = append(requesterValues, cedar.TriggerRequester)
		case htdAPIRequesterGitTag:
			requesterValues = append(requesterValues, cedar.GitTagRequester)
		case htdAPIRequesterAdhoc:
			requesterValues = append(requesterValues, cedar.AdHocRequester)
		default:
			return nil, errors.Errorf("invalid requester value %v", requester)
		}
	}
	return requesterValues, nil
}

// readStringList parses a string list parameter value, the values can be comma
// separated or specified multiple times.
func (h *htdFilterHandler) readStringList(values []string) []string {
	var parsedValues []string
	for _, val := range values {
		elements := strings.Split(val, ",")
		parsedValues = append(parsedValues, elements...)
	}
	return parsedValues
}

// readInt parses an integer parameter value, given minimum, maximum, and
// default values.
func (h *htdFilterHandler) readInt(intString string, min, max, defaultValue int) (int, error) {
	if intString == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(intString)
	if err != nil {
		return 0, err
	}

	if value < min || value > max {
		return 0, errors.New("invalid int parameter value")
	}
	return value, nil
}

// readTestGroupBy parses a sort parameter value and returns the corresponding
// HTDSort struct.
func (h *htdFilterHandler) readSort(sortValue string) (model.HTDSort, error) {
	switch sortValue {
	case htdAPISortEarliest:
		return model.HTDSortEarliestFirst, nil
	case htdAPISortLatest:
		return model.HTDSortLatestFirst, nil
	case "":
		return model.HTDSortEarliestFirst, nil
	default:
		return model.HTDSort(""), gimlet.ErrorResponse{
			Message:    "invalid sort value",
			StatusCode: http.StatusBadRequest,
		}
	}
}

// readGroupBy parses a group_by parameter value and returns the corresponding
// HTDGroupBy struct.
func (h *htdFilterHandler) readGroupBy(groupByValue string) (model.HTDGroupBy, error) {
	switch groupByValue {
	case htdAPITestGroupByVariant:
		return model.HTDGroupByVariant, nil
	case htdAPITestGroupByTask:
		return model.HTDGroupByTask, nil
	case htdAPITestGroupByTest:
		return model.HTDGroupByTest, nil
	// We no longer store distro, but do not want to error if it is
	// present.
	case "", htdAPITestGroupByDistro:
		return model.HTDGroupByVariant, nil
	default:
		return model.HTDGroupBy(""), gimlet.ErrorResponse{
			Message:    "invalid group_by value",
			StatusCode: http.StatusBadRequest,
		}
	}
}

// readStartAt parses a start_at key value and returns the corresponding
// HTDStartAt struct.
func (h *htdFilterHandler) readStartAt(startAtValue string) (*model.HTDStartAt, error) {
	if startAtValue == "" {
		return nil, nil
	}

	elements := strings.Split(startAtValue, "|")
	if len(elements) != 4 && len(elements) != 5 {
		return nil, gimlet.ErrorResponse{
			Message:    "invalid start_at value",
			StatusCode: http.StatusBadRequest,
		}
	}

	date, err := time.ParseInLocation(htdAPIDateFormat, elements[0], time.UTC)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			Message:    "invalid start_at value",
			StatusCode: http.StatusBadRequest,
		}
	}

	return &model.HTDStartAt{
		Date:    date,
		Variant: elements[1],
		Task:    elements[2],
		Test:    elements[3],
	}, nil
}
