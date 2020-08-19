package rest

import (
	"context"
	"net/http"
	"strconv"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /system_metrics/type/{task_id}/{type}

type systemMetricsGetByTypeHandler struct {
	metricType string
	opts       model.SystemMetricsFindOptions
	sc         data.Connector
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

	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.metricType = gimlet.GetVars(r)["type"]
	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		if err != nil {
			return err
		}
	} else {
		h.opts.EmptyExecution = true
	}

	return nil
}

// Run finds and returns the desired system metric data based on task id and
// metric type.
func (h *systemMetricsGetByTypeHandler) Run(ctx context.Context) gimlet.Responder {
	data, err := h.sc.FindSystemMetricsByType(ctx, h.metricType, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting metric type '%s' for task id '%s'", h.metricType, h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":     gimlet.GetRequestID(ctx),
			"method":      "GET",
			"route":       "/system_metrics/type/{task_id}/{type}",
			"task_id":     h.opts.TaskID,
			"metric_type": h.metricType,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return gimlet.NewTextResponse(data)
}
