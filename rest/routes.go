package rest

import (
	"net/http"

	"github.com/mongodb/amboy"
	"github.com/tychoish/gimlet"
	"github.com/tychoish/sink"
)

type statusResponse struct {
	Revision     string           `json:"revision"`
	QueueStats   amboy.QueueStats `json:"queue,omitempty"`
	QueueRunning bool             `json:"running"`
}

func (s *Service) statusHandler(w http.ResponseWriter, r *http.Request) {
	resp := &statusResponse{Revision: sink.BuildRevision}

	if s.queue != nil {
		resp.QueueRunning = s.queue.Started()
		resp.QueueStats = s.queue.Stats()
	}

	gimlet.WriteJSON(w, resp)
}
