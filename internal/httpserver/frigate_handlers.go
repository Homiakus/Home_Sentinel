package httpserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

func (s *Server) frigateStatus(w http.ResponseWriter, r *http.Request) {
	if s.app.Frigate == nil {
		writeProblem(w, r, 503, "FRIGATE_DISABLED", "Frigate integration is not enabled")
		return
	}
	caps, diag, err := s.app.Frigate.Capabilities(r.Context())
	if err != nil {
		writeProblem(w, r, 502, "FRIGATE_UNAVAILABLE", "Unable to reach Frigate")
		return
	}
	writeJSON(w, 200, map[string]any{"capabilities": caps, "diagnostic": diag})
}
func (s *Server) frigatePlan(w http.ResponseWriter, r *http.Request) {
	if s.app.Frigate == nil {
		writeProblem(w, r, 503, "FRIGATE_DISABLED", "Frigate integration is not enabled")
		return
	}
	p, err := s.app.Frigate.Plan(r.Context())
	if err != nil {
		writeProblem(w, r, 422, "FRIGATE_PLAN_FAILED", "Unable to produce a valid Frigate plan")
		return
	}
	writeJSON(w, 200, map[string]any{"version": p.Version, "ownership": p.Ownership, "checksum": p.Checksum, "preflight": p.Preflight, "secret_count": len(p.SecretEnv)})
}
func (s *Server) frigateApply(w http.ResponseWriter, r *http.Request) {
	if s.app.Frigate == nil {
		writeProblem(w, r, 503, "FRIGATE_DISABLED", "Frigate integration is not enabled")
		return
	}
	res, err := s.app.Frigate.Apply(r.Context())
	p, _ := principalFrom(r.Context())
	result := "success"
	if err != nil {
		result = "failed"
	}
	if s.app.Audit != nil {
		details, _ := json.Marshal(map[string]any{"checksum": res.Checksum, "rolled_back": res.RolledBack})
		_, _ = s.app.Audit.Append(r.Context(), repository.AuditEntry{OccurredAt: time.Now().UTC(), Actor: p.User.ID, Source: "web", Action: "frigate.apply", Target: "frigate", Result: result, RequestID: requestIDFrom(r.Context()), Details: details})
	}
	if err != nil {
		writeProblem(w, r, 502, "FRIGATE_APPLY_FAILED", "Frigate configuration was not applied; rollback status is recorded in audit")
		return
	}
	writeJSON(w, 200, res)
}
func (s *Server) frigateDrift(w http.ResponseWriter, r *http.Request) {
	if s.app.Frigate == nil {
		writeProblem(w, r, 503, "FRIGATE_DISABLED", "Frigate integration is not enabled")
		return
	}
	report, err := s.app.Frigate.Reconcile(r.Context())
	if err != nil {
		writeProblem(w, r, 502, "FRIGATE_RECONCILE_FAILED", "Unable to reconcile Frigate state")
		return
	}
	writeJSON(w, 200, report)
}
func (s *Server) frigateEvents(w http.ResponseWriter, r *http.Request) {
	if s.app.Frigate == nil {
		writeProblem(w, r, 503, "FRIGATE_DISABLED", "Frigate integration is not enabled")
		return
	}
	q := url.Values{}
	for _, k := range []string{"camera", "label", "after", "before"} {
		if v := r.URL.Query().Get(k); v != "" {
			q.Set(k, v)
		}
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	q.Set("limit", strconv.Itoa(limit))
	items, err := s.app.Frigate.Events().List(r.Context(), q)
	if err != nil {
		writeProblem(w, r, 502, "FRIGATE_EVENTS_FAILED", "Unable to read Frigate events")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (s *Server) frigateEvent(w http.ResponseWriter, r *http.Request) {
	if s.app.Frigate == nil {
		writeProblem(w, r, 503, "FRIGATE_DISABLED", "Frigate integration is not enabled")
		return
	}
	item, err := s.app.Frigate.Events().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProblem(w, r, 502, "FRIGATE_EVENT_FAILED", "Unable to read Frigate event")
		return
	}
	writeJSON(w, 200, item)
}
