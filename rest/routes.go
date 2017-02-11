package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mongodb/amboy"
	"github.com/tychoish/gimlet"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sink"
	"github.com/tychoish/sink/model"
	"github.com/tychoish/sink/units"
)

////////////////////////////////////////////////////////////////////////
//
// GET /status

type StatusResponse struct {
	Revision     string           `json:"revision"`
	QueueStats   amboy.QueueStats `json:"queue,omitempty"`
	QueueRunning bool             `json:"running"`
}

// statusHandler processes the GET request for
func (s *Service) statusHandler(w http.ResponseWriter, r *http.Request) {
	resp := &StatusResponse{Revision: sink.BuildRevision}

	if s.queue != nil {
		resp.QueueRunning = s.queue.Started()
		resp.QueueStats = s.queue.Stats()
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /status/events/{level}limit=<int>

type SystemEvents struct {
	Level  string        `json:"level,omitempty"`
	Count  int           `json:"count,omitempty"`
	Events []model.Event `json:"events"`
	Err    string        `json:"error"`
}

func (s *Service) getSystemEvents(w http.ResponseWriter, r *http.Request) {
	l := gimlet.GetVars(r)["level"]
	resp := &SystemEvents{}

	if l == "" {
		resp.Err = "no level specified"
		grip.WriteErrorJSON(w, resp)
		return
	}

	if !level.IsValidPriority(level.FromString(l)) {
		resp.Err = fmt.Sprintf("%s is not a valid level", l)
	}
	resp.Level = l

	limitArg := r.URL.Query()["limit"]
	limit, err := strconv.Atoi(limitArg)
	if err != nil {
		resp.Err = fmt.Sprintf("%s is not a valid limit [%s]", limitArg, err.Error())
		grip.WriteErrorJSON(w, resp)
		return
	}

	e := &model.Events{}
	err = e.FindLevel(l, limit)
	if err != nil {
		resp.Err = "problem running query for events"
		gimlet.WriteInternalErrorJSON(w, resp)
		return
	}

	resp.Events = e.Events()
	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// POST /simple_log/{id}
//
// body: { "inc": <int>, "ts": <date>, "content": <str> }

type simpleLogRequest struct {
	Time      time.Time `json:"ts"`
	Increment int       `json:"inc"`
	Content   string    `json:"content"`
}

type SimpleLogInjestionResponse struct {
	Errors []string `json:"errors,omitempty"`
	JobID  string   `json:"jobId,omitempty"`
	LogID  string   `json:"logId"`
}

func (s *Service) simpleLogInjestion(w http.ResponseWriter, r *http.Request) {
	req := &simpleLogRequest{}
	resp := &SimpleLogInjestionResponse{}
	resp.LogID = gimlet.GetVars(r)["id"]
	defer r.Body.Close()

	if resp.LogID == "" {
		resp.Errors = []string{"no log id specified"}
		gimlet.WriteErrorJSON(w, resp)
		return
	}

	if err := gimlet.GetJSON(r.Body, req); err != nil {
		grip.Error(err)
		resp.Errors = append(resp.Errors, err.Error())
		gimlet.WriteErrorJSON(w, resp)
		return
	}

	j := units.MakeSaveSimpleLogJob(resp.LogID, req.Content, req.Time, req.Increment)
	resp.JobID = j.ID()

	if err := s.queue.Put(j); err != nil {
		grip.Error(err)
		resp.Errors = append(resp.Errors, err.Error())
		gimlet.WriteInternalErrorJSON(w, resp)
		return
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /simple_log/{id}

type SimpleLogContentResponse struct {
	LogID string   `json:"logId"`
	Error string   `json:"err,omitempty"`
	URLS  []string `json:"urls"`
}

// simpleLogRetrieval takes in a log id and returns the log documents associated with that log id.
func (s *Service) simpleLogRetrieval(w http.ResponseWriter, r *http.Request) {
	resp := &SimpleLogContentResponse{}

	resp.LogID = gimlet.GetVars(r)["id"]
	if resp.LogID == "" {
		resp.Error = "no log specified"
		gimlet.WriteErrorJSON(w, resp)
		return
	}
	allLogs := &model.LogSegments{}

	if err := allLogs.Find(resp.LogID, false); err != nil {
		resp.Error = err.Error()
		gimlet.WriteErrorJSON(w, resp)
		return
	}

	for _, l := range allLogs.LogSegments() {
		resp.URLS = append(resp.URLS, l.URL)
	}

	gimlet.WriteJSON(w, resp)
}
