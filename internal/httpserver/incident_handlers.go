package httpserver

import (
	"errors"
	"net/http"

	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

func (s *Server) incidentList(w http.ResponseWriter, r *http.Request) {
	if s.app.Incidents == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "INCIDENTS_UNAVAILABLE", "Incident service unavailable")
		return
	}
	items, err := s.app.Incidents.List(r.Context(), 100)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INCIDENT_LIST_FAILED", "Unable to list incidents")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) incidentGet(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.Incidents.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", "Incident not found")
		return
	}
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INCIDENT_READ_FAILED", "Unable to read incident")
		return
	}
	eventsOut := make([]any, 0, len(item.Value.EventIDs))
	for _, id := range item.Value.EventIDs {
		if ev, err := s.app.Incidents.Event(r.Context(), id.String()); err == nil {
			eventsOut = append(eventsOut, ev)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"incident": item, "events": eventsOut})
}
