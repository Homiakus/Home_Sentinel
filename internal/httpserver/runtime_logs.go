package httpserver

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/authz"
	"github.com/Homiakus/Home_Sentinel/internal/telemetry"
)

const (
	runtimeLogDefaultLimit = 250
	runtimeLogMaxLimit     = 4096
	settingsLogsScriptTag  = "  <script type=\"module\" src=\"/assets/settings-logs.js\"></script>\n"
)

// EnableRuntimeLogs adds an admin-only runtime-log endpoint and augments the
// embedded settings UI without changing the public server constructor contract.
// It must be called before ListenAndServe.
func (s *Server) EnableRuntimeLogs(logs *telemetry.LogBuffer) {
	if s == nil || s.http == nil || logs == nil {
		return
	}
	base := s.http.Handler
	logsHandler := requestID(securityHeaders(s.observeHTTP(s.authRequired(s.requireCapability(
		authz.ChangeConfig,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.runtimeLogs(w, r, logs) }),
	)))))
	indexHandler := requestID(securityHeaders(s.observeHTTP(http.HandlerFunc(s.settingsLogsIndex))))

	s.http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/system/logs" {
			logsHandler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			indexHandler.ServeHTTP(w, r)
			return
		}
		base.ServeHTTP(w, r)
	})
}

func (s *Server) runtimeLogs(w http.ResponseWriter, r *http.Request, logs *telemetry.LogBuffer) {
	limit := runtimeLogDefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > runtimeLogMaxLimit {
			writeProblem(w, r, http.StatusBadRequest, "LOG_LIMIT_INVALID", "Log limit must be between 1 and 4096")
			return
		}
		limit = parsed
	}
	snapshot := logs.Snapshot(limit)
	lines := snapshot.Lines
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"captured_at": time.Now().UTC(),
		"lines":       lines,
		"retained":    snapshot.Retained,
		"capacity":    snapshot.Capacity,
		"returned":    len(lines),
		"max_limit":   runtimeLogMaxLimit,
	})
}

func (s *Server) settingsLogsIndex(w http.ResponseWriter, r *http.Request) {
	b, err := uiFiles.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	marker := []byte("</body>")
	if !bytes.Contains(b, marker) {
		http.Error(w, "UI template invalid", http.StatusInternalServerError)
		return
	}
	b = bytes.Replace(b, marker, append([]byte(settingsLogsScriptTag), marker...), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}
