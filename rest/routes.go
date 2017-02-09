package rest

import (
	"net/http"
	"time"

	"github.com/mongodb/amboy"
	"github.com/tychoish/gimlet"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink"
	"github.com/tychoish/sink/units"
)

////////////////////////////////////////////////////////////////////////
//
// GET /status

type statusResponse struct {
	Revision     string           `json:"revision"`
	QueueStats   amboy.QueueStats `json:"queue,omitempty"`
	QueueRunning bool             `json:"running"`
}

// statusHandler processes the GET request for
func (s *Service) statusHandler(w http.ResponseWriter, r *http.Request) {
	resp := &statusResponse{Revision: sink.BuildRevision}

	if s.queue != nil {
		resp.QueueRunning = s.queue.Started()
		resp.QueueStats = s.queue.Stats()
	}

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

type simpleLogInjestionResponse struct {
	Errors []string `json:"errors,omitempty"`
	JobID  string   `json:"jobId,omitempty"`
	LogID  string   `json:"logId"`
}

func (s *Service) simpleLogInjestion(w http.ResponseWriter, r *http.Request) {
	req := &simpleLogRequest{}
	resp := &simpleLogInjestionResponse{}
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

type simpleLogContentResponse struct {
	LogID string   `json:"logId"`
	Error string   `json:"err,omitempty"`
	URLS  []string `json:"urls"`
}

func (s *Service) simpleLogRetrieval(w http.ResponseWriter, r *http.Request) {
	resp := &simpleLogContentResponse{}

	resp.LogID = gimlet.GetVars(r)["id"]
	if resp.LogID == "" {
		resp.Error = "no log specified"
		gimlet.WriteErrorJSON(w, resp)
		return
	}

	// TODO add more data and populate it from the database

	gimlet.WriteJSON(w, resp)
}
