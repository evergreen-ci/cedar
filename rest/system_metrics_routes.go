package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

var startIndex = "start_index"

///////////////////////////////////////////////////////////////////////////////
//
// GET /system_metrics/type/{task_id}/{type}

type systemMetricsGetByTypeHandler struct {
	metricType   string
	findOpts     model.SystemMetricsFindOptions
	downloadOpts model.SystemMetricsDownloadOptions
	sc           data.Connector
}

func makeGetSystemMetricsByType(sc data.Connector) gimlet.RouteHandler {
	return &systemMetricsGetByTypeHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new systemMetricsGetByTypeHandler.
func (h *systemMetricsGetByTypeHandler) Factory() gimlet.RouteHandler {
	return &systemMetricsGetByTypeHandler{
		sc: h.sc,
	}
}

// Parse fetches the task_id and metric type from the http request.
func (h *systemMetricsGetByTypeHandler) Parse(_ context.Context, r *http.Request) error {
	var err error

	h.findOpts.TaskID = gimlet.GetVars(r)["task_id"]
	h.downloadOpts.MetricType = gimlet.GetVars(r)["type"]
	vals := r.URL.Query()
	h.downloadOpts.PageSize = softSizeLimit
	if len(vals[execution]) > 0 {
		h.findOpts.Execution, err = strconv.Atoi(vals[execution][0])
		if err != nil {
			return err
		}
	} else {
		h.findOpts.EmptyExecution = true
	}
	if len(vals[startIndex]) > 0 {
		h.downloadOpts.StartIndex, err = strconv.Atoi(vals[startIndex][0])
		if err != nil {
			return err
		}
	}

	return nil
}

// Run finds and returns the desired system metric data based on task id and
// metric type.
func (h *systemMetricsGetByTypeHandler) Run(ctx context.Context) gimlet.Responder {
	data, nextIdx, err := h.sc.FindSystemMetricsByType(ctx, h.findOpts, h.downloadOpts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting metric type '%s' for task id '%s'", h.downloadOpts.MetricType, h.findOpts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":     gimlet.GetRequestID(ctx),
			"method":      "GET",
			"route":       "/system_metrics/type/{task_id}/{type}",
			"task_id":     h.findOpts.TaskID,
			"metric_type": h.downloadOpts.MetricType,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return newSystemMetricsResponder(h.sc.GetBaseURL(), data, h.downloadOpts.StartIndex, nextIdx)
}

func newSystemMetricsResponder(baseURL string, data []byte, startIdx, nextIdx int) gimlet.Responder {
	resp := gimlet.NewTextResponse(data)

	pages := &gimlet.ResponsePages{
		Prev: &gimlet.Page{
			BaseURL:         baseURL,
			KeyQueryParam:   startIndex,
			LimitQueryParam: limit,
			Key:             fmt.Sprintf("%d", startIdx),
			Relation:        "prev",
		},
	}
	if startIdx != nextIdx {
		pages.Next = &gimlet.Page{
			BaseURL:         baseURL,
			KeyQueryParam:   startIndex,
			LimitQueryParam: limit,
			Key:             fmt.Sprintf("%d", nextIdx),
			Relation:        "next",
		}
	}

	if err := resp.SetPages(pages); err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrap(err, "problem setting response pages"))
	}

	return resp
}
