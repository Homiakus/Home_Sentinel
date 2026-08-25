package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/authz"
	"github.com/Homiakus/Home_Sentinel/internal/buildinfo"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/realtime"
)

type Server struct {
	app         *app.App
	http        *http.Server
	limiter     *rateLimiter
	go2rtcProxy *httputil.ReverseProxy
}

func New(a *app.App) *Server {
	mux := http.NewServeMux()
	s := &Server{app: a, limiter: newRateLimiter()}
	if a.Config.Frigate.Go2RTCURL != "" {
		if target, err := validateGo2RTCURL(a.Config.Frigate.Go2RTCURL); err == nil {
			s.go2rtcProxy = httputil.NewSingleHostReverseProxy(target)
			originalDirector := s.go2rtcProxy.Director
			s.go2rtcProxy.Director = func(req *http.Request) {
				originalDirector(req)
				// Sentinel credentials authenticate the browser to Sentinel only. Never
				// forward them to the internal go2rtc process.
				req.Header.Del("Cookie")
				req.Header.Del("Authorization")
				req.Header.Del("X-CSRF-Token")
			}
			s.go2rtcProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Set-Cookie")
				return nil
			}
			s.go2rtcProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				a.Log.Warn("go2rtc proxy error", "error", err)
				writeProblem(w, r, http.StatusBadGateway, "GO2RTC_PROXY_FAILED", "Live stream backend is unavailable")
			}
		}
	}
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/system", s.system)
	mux.Handle("GET /api/v1/health", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.healthDetail))))
	mux.HandleFunc("GET /api/v1/setup/status", s.setupStatus)
	mux.Handle("POST /api/v1/setup/admin", s.rateLimit("setup-admin", 5, time.Minute, http.HandlerFunc(s.setupAdmin)))
	mux.Handle("POST /api/v1/auth/login", s.rateLimit("login", 10, time.Minute, http.HandlerFunc(s.login)))
	mux.Handle("GET /api/v1/auth/me", s.authRequired(http.HandlerFunc(s.me)))
	mux.Handle("GET /api/v1/auth/csrf", s.authRequired(http.HandlerFunc(s.csrf)))
	mux.Handle("GET /api/v1/hardware", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.hardware))))
	mux.Handle("GET /api/v1/setup/wizard", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.setupWizard))))
	mux.Handle("POST /api/v1/setup/verify", s.authRequired(s.csrfRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.setupVerify)))))
	mux.Handle("POST /api/v1/auth/logout", s.authRequired(s.csrfRequired(http.HandlerFunc(s.logout))))
	mux.Handle("POST /api/v1/auth/reauth", s.rateLimit("reauth", 10, time.Minute, s.authRequired(s.csrfRequired(http.HandlerFunc(s.reauthenticate)))))
	mux.Handle("GET /api/v1/users", s.authRequired(s.requireCapability(authz.ManageUsers, http.HandlerFunc(s.userList))))
	mux.Handle("POST /api/v1/users", s.authRequired(s.csrfRequired(s.requireCapability(authz.ManageUsers, s.freshAuthentication(15*time.Minute, http.HandlerFunc(s.userCreate))))))
	mux.Handle("PATCH /api/v1/users/{id}", s.authRequired(s.csrfRequired(s.requireCapability(authz.ManageUsers, s.freshAuthentication(15*time.Minute, http.HandlerFunc(s.userAccessUpdate))))))
	mux.Handle("GET /api/v1/events/stream", s.authRequired(http.HandlerFunc(s.stream)))
	mux.Handle("GET /api/v1/events", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.eventFeed))))
	mux.Handle("GET /api/v1/dashboard/overview", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.dashboardOverview))))
	mux.Handle("GET /api/v1/search", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.search))))
	mux.Handle("GET /api/v1/system/diagnostics", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.systemDiagnostics))))
	mux.Handle("GET /api/v1/cameras", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.cameraList))))
	mux.Handle("GET /api/v1/cameras/{id}", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.cameraGet))))
	mux.Handle("GET /api/v1/cameras/{id}/live", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.cameraLiveDescriptor))))
	mux.Handle("GET /api/v1/cameras/{id}/diagnostics", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.cameraDiagnostics))))
	mux.Handle("GET /api/v1/media/cameras/{id}/latest.jpg", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.cameraLatest))))
	mux.Handle("GET /api/v1/media/events/{id}/snapshot.jpg", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.eventSnapshot))))
	mux.Handle("GET /api/v1/media/events/{id}/clip.mp4", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.eventClip))))
	mux.Handle("POST /api/v1/cameras/onboard/rtsp", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.cameraOnboardRTSP)))))
	mux.Handle("GET /api/v1/cameras/discover/onvif", s.authRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.cameraDiscoverONVIF))))
	mux.Handle("POST /api/v1/cameras/onboard/onvif", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.cameraOnboardONVIF)))))
	mux.Handle("GET /api/v1/cameras/discover/uvc", s.authRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.cameraDiscoverUVC))))
	mux.Handle("POST /api/v1/cameras/onboard/uvc", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.cameraOnboardUVC)))))
	mux.Handle("GET /api/v1/frigate/status", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.frigateStatus))))
	mux.Handle("GET /api/v1/frigate/plan", s.authRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.frigatePlan))))
	mux.Handle("POST /api/v1/frigate/apply", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, s.freshAuthentication(15*time.Minute, http.HandlerFunc(s.frigateApply))))))
	mux.Handle("GET /api/v1/frigate/drift", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.frigateDrift))))
	mux.Handle("GET /api/v1/frigate/events", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.frigateEvents))))
	mux.Handle("GET /api/v1/frigate/events/{id}", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.frigateEvent))))
	mux.Handle("GET /api/v1/incidents", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.incidentList))))
	mux.Handle("GET /api/v1/incidents/{id}", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.incidentGet))))
	mux.Handle("GET /api/v1/homeassistant/status", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.homeAssistantStatus))))
	mux.Handle("GET /api/v1/setup/homeassistant", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.homeAssistantSetupGet))))
	mux.Handle("POST /api/v1/setup/homeassistant/probe", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.homeAssistantProbe)))))
	mux.Handle("POST /api/v1/setup/homeassistant", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, s.freshAuthentication(15*time.Minute, http.HandlerFunc(s.homeAssistantConfigure))))))
	mux.Handle("POST /api/v1/homeassistant/verify-mqtt", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.homeAssistantVerifyMQTT)))))
	mux.Handle("GET /api/v1/homeassistant/verify-frigate", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.homeAssistantVerifyFrigate))))
	mux.Handle("GET /api/v1/intercoms", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.intercomList))))
	mux.Handle("GET /api/v1/intercoms/{id}", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.intercomGet))))
	mux.Handle("PUT /api/v1/intercoms", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.intercomPut)))))
	mux.Handle("POST /api/v1/intercoms/{id}/unlock", s.rateLimit("unlock", 10, time.Minute, s.authRequired(s.csrfRequired(s.requireCapability(authz.UnlockDoor, s.freshAuthentication(15*time.Minute, http.HandlerFunc(s.intercomUnlock)))))))
	mux.Handle("GET /api/v1/intercoms/{id}/commands/{request_id}", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.intercomCommand))))
	mux.Handle("GET /api/v1/ai/status", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.aiStatus))))
	mux.Handle("GET /api/v1/ai/models", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.aiModels))))
	mux.Handle("GET /api/v1/cameras/{id}/ai-policy", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.aiPolicyGet))))
	mux.Handle("PUT /api/v1/cameras/{id}/ai-policy", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, http.HandlerFunc(s.aiPolicyPut)))))
	mux.Handle("GET /api/v1/telegram/status", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.telegramStatus))))
	mux.Handle("POST /api/v1/telegram/pairing", s.rateLimit("telegram-pairing", 10, time.Hour, s.authRequired(s.csrfRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.telegramPairing))))))
	mux.Handle("GET /api/v1/backups/status", s.authRequired(s.requireCapability(authz.ViewSystem, http.HandlerFunc(s.backupStatus))))
	mux.Handle("POST /api/v1/backups/init", s.authRequired(s.csrfRequired(s.requireCapability(authz.RunBackup, http.HandlerFunc(s.backupInit)))))
	mux.Handle("POST /api/v1/backups/run", s.authRequired(s.csrfRequired(s.requireCapability(authz.RunBackup, http.HandlerFunc(s.backupRun)))))
	mux.Handle("POST /api/v1/backups/check", s.authRequired(s.csrfRequired(s.requireCapability(authz.RunBackup, http.HandlerFunc(s.backupCheck)))))
	mux.Handle("POST /api/v1/backups/restore-test", s.authRequired(s.csrfRequired(s.requireCapability(authz.RunBackup, http.HandlerFunc(s.backupRestoreTest)))))
	mux.Handle("POST /api/v1/backups/retention/preview", s.authRequired(s.csrfRequired(s.requireCapability(authz.RunBackup, http.HandlerFunc(s.backupRetentionPreview)))))
	mux.Handle("POST /api/v1/backups/retention/apply", s.authRequired(s.csrfRequired(s.requireCapability(authz.ChangeConfig, s.freshAuthentication(15*time.Minute, http.HandlerFunc(s.backupRetentionApply))))))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", uiHandler()))
	mux.Handle("GET /stream.html", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.go2rtc))))
	mux.Handle("GET /webrtc.html", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.go2rtc))))
	mux.Handle("GET /video-rtc.js", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.go2rtc))))
	mux.Handle("GET /video-stream.js", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.go2rtc))))
	mux.Handle("GET /api/ws", s.authRequired(s.requireCapability(authz.ViewLive, http.HandlerFunc(s.go2rtc))))
	mux.HandleFunc("GET /", s.index)
	handler := requestID(securityHeaders(s.observeHTTP(mux)))
	s.http = &http.Server{Addr: a.Config.Server.ListenAddress, Handler: handler, ReadTimeout: a.Config.Server.ReadTimeout, WriteTimeout: a.Config.Server.WriteTimeout, IdleTimeout: 60 * time.Second, ReadHeaderTimeout: 5 * time.Second}
	return s
}
func (s *Server) Handler() http.Handler              { return s.http.Handler }
func (s *Server) ListenAndServe() error              { return s.http.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "HEALTHY", "component": "sentinel-core"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if !s.app.Ready(r.Context()) {
		writeJSON(w, 503, map[string]any{"status": "STARTING"})
		return
	}
	writeJSON(w, 200, map[string]any{"status": "READY"})
}
func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	var schema int64
	if s.app.DB != nil {
		var err error
		schema, err = database.SchemaVersion(r.Context(), s.app.DB)
		if err != nil {
			writeProblem(w, r, http.StatusServiceUnavailable, "SCHEMA_UNAVAILABLE", "Database schema version is unavailable")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"name": "Home Sentinel", "version": buildinfo.Version, "commit": buildinfo.Commit, "schema_version": schema, "started_at": s.app.StartedAt()})
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, r, 500, "STREAM_UNSUPPORTED", "Streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	// The server-wide WriteTimeout protects ordinary HTTP handlers, but SSE is
	// intentionally long-lived. Clear the per-response deadline while keeping
	// bounded subscriber queues and client-context cancellation.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	ch, cancel := s.app.Realtime.Subscribe(64)
	defer cancel()
	keep := time.NewTicker(20 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(m)
			fmt.Fprintf(w, "id: %s\ndata: %s\n\n", m.ID, b)
			flusher.Flush()
		case <-keep.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
func (s *Server) PublishTestEvent() error {
	id, err := domain.NewID("evt")
	if err != nil {
		return err
	}
	s.app.Realtime.Publish(realtime.Message{ID: id.String(), Type: "system.test", Data: json.RawMessage(`{"ok":true}`)})
	return nil
}

func (s *Server) hardware(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"profile": s.app.Hardware, "recommendation": s.app.HardwareRecommendation})
}
