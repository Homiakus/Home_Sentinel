package httpserver

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/health"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusRecorder) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

func (s *Server) observeHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		if s.app.Metrics != nil {
			s.app.Metrics.ObserveHTTP(rw.status)
		}
	})
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	m := s.app.Metrics
	var req, errs uint64
	if m != nil {
		req, errs = m.HTTPRequests.Load(), m.HTTPErrors.Load()
	}
	fmt.Fprintf(w, "# TYPE sentinel_http_requests_total counter\nsentinel_http_requests_total %d\n", req)
	fmt.Fprintf(w, "# TYPE sentinel_http_errors_total counter\nsentinel_http_errors_total %d\n", errs)
	if s.app.Realtime != nil {
		subs, dropped := s.app.Realtime.Stats()
		fmt.Fprintf(w, "# TYPE sentinel_realtime_subscribers gauge\nsentinel_realtime_subscribers %d\n", subs)
		fmt.Fprintf(w, "# TYPE sentinel_realtime_dropped_total counter\nsentinel_realtime_dropped_total %d\n", dropped)
	}
	if s.app.Health != nil {
		fmt.Fprintln(w, "# TYPE sentinel_component_health gauge")
		for _, c := range s.app.Health.Snapshot() {
			fmt.Fprintf(w, "sentinel_component_health{component=\"%s\"} %g\n", prometheusLabel(c.Name), healthValue(c.Status))
		}
	}
	if s.app.Cameras != nil {
		if cams, err := s.app.Cameras.List(r.Context(), 1000); err == nil {
			fmt.Fprintf(w, "# TYPE sentinel_cameras_configured gauge\nsentinel_cameras_configured %d\n", len(cams))
		}
	}
	if s.app.Backup != nil && s.app.Backup.Jobs != nil {
		if jobs, err := s.app.Backup.Jobs.List(r.Context(), 100); err == nil {
			var last time.Time
			for _, j := range jobs {
				if j.Value.Status == "succeeded" && j.Value.FinishedAt.After(last) {
					last = j.Value.FinishedAt
				}
			}
			if !last.IsZero() {
				fmt.Fprintf(w, "# TYPE sentinel_backup_last_success_timestamp_seconds gauge\nsentinel_backup_last_success_timestamp_seconds %d\n", last.Unix())
			}
		}
	}
}

func healthValue(s health.Status) float64 {
	switch s {
	case health.Healthy:
		return 1
	case health.Starting:
		return .75
	case health.Degraded:
		return .5
	case health.Failed:
		return 0
	default:
		return .25
	}
}
func prometheusLabel(v string) string {
	return strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n").Replace(v)
}
