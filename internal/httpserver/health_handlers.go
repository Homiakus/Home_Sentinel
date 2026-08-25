package httpserver

import (
	"net/http"

	"github.com/Homiakus/Home_Sentinel/internal/health"
)

var dependencyGraph = health.DependencyGraph{
	"home_assistant": {"mqtt"},
	"intercom":       {"mqtt"},
	"telegram":       {"sentinel"},
	"ai":             {"sentinel"},
	"backup":         {"database"},
	"frigate":        {"sentinel"},
}

func (s *Server) healthDetail(w http.ResponseWriter, r *http.Request) {
	if s.app.Health == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": health.Unknown, "components": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     s.app.Health.Overall(),
		"components": health.Diagnose(s.app.Health, dependencyGraph),
	})
}
