package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/integrations/go2rtc"
)

func (s *Server) eventSnapshot(w http.ResponseWriter, r *http.Request) {
	s.proxyFrigateMedia(w, r, "events/"+r.PathValue("id")+"/snapshot.jpg", nil, "private, max-age=30")
}

func (s *Server) eventClip(w http.ResponseWriter, r *http.Request) {
	s.proxyFrigateMedia(w, r, "events/"+r.PathValue("id")+"/clip.mp4", nil, "private, max-age=300")
}

func (s *Server) cameraLatest(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "CAMERAS_UNAVAILABLE", "Camera service unavailable")
		return
	}
	cam, err := s.app.Cameras.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProblem(w, r, http.StatusNotFound, "CAMERA_NOT_FOUND", "Camera not found")
		return
	}
	if s.app.Frigate != nil && s.app.Frigate.Client != nil {
		name := go2rtc.CanonicalStreamName(cam.ID, cameras.RoleMain)
		s.proxyFrigateMedia(w, r, name+"/latest.jpg", nil, "no-store")
		return
	}
	st, ok := cam.StreamByRole(cameras.RoleMain)
	if !ok && len(cam.Streams) > 0 {
		st = cam.Streams[0]
	}
	input := st.Endpoint.URL
	if input == "" {
		writeProblem(w, r, http.StatusServiceUnavailable, "STREAM_UNAVAILABLE", "Camera stream URL is not configured")
		return
	}
	shot, err := cameras.Snapshot(r.Context(), input, 5*time.Second)
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "SNAPSHOT_FAILED", "Failed to capture snapshot: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(shot)
}

func (s *Server) proxyFrigateMedia(w http.ResponseWriter, r *http.Request, path string, q url.Values, cache string) {
	if s.app.Frigate == nil || s.app.Frigate.Client == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "FRIGATE_DISABLED", "Frigate media is unavailable")
		return
	}
	resp, err := s.app.Frigate.Client.OpenMedia(r.Context(), path, q)
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "FRIGATE_MEDIA_FAILED", "Unable to read Frigate media")
		return
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	if cache != "" {
		w.Header().Set("Cache-Control", cache)
	}
	w.Header().Set("Accept-Ranges", "none")
	if _, err := io.CopyBuffer(w, resp.Body, make([]byte, 64<<10)); err != nil {
		s.app.Log.Debug("media proxy interrupted", "path", path, "error", err)
	}
}

func (s *Server) go2rtc(w http.ResponseWriter, r *http.Request) {
	if s.go2rtcProxy == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "GO2RTC_PROXY_DISABLED", "go2rtc live proxy is not configured")
		return
	}
	path := r.URL.Path
	allowed := path == "/stream.html" || path == "/webrtc.html" || path == "/video-rtc.js" || path == "/video-stream.js" || path == "/api/ws"
	if !allowed {
		http.NotFound(w, r)
		return
	}
	if path == "/stream.html" || path == "/webrtc.html" {
		src := strings.TrimSpace(r.URL.Query().Get("src"))
		if src == "" || len(src) > 160 || strings.ContainsAny(src, "/\\?&#") {
			writeProblem(w, r, http.StatusBadRequest, "INVALID_STREAM", "Invalid live stream name")
			return
		}
		// A stream may only be viewed if it maps to a currently configured camera.
		found := false
		if cams, err := s.app.Cameras.List(r.Context(), 500); err == nil {
			for _, c := range cams {
				if go2rtc.CanonicalStreamName(c.ID, cameras.RoleMain) == src || go2rtc.CanonicalStreamName(c.ID, cameras.RoleDetect) == src {
					found = true
					break
				}
			}
		}
		if !found {
			writeProblem(w, r, http.StatusNotFound, "STREAM_NOT_FOUND", "Stream is not managed by Sentinel")
			return
		}
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data: https://go2rtc.org; frame-ancestors 'self'; base-uri 'none'")
	}
	// go2rtc's /api/ws carries src in its query; require a managed stream so the
	// authenticated proxy cannot be used to reach arbitrary go2rtc sources.
	if path == "/api/ws" {
		src := strings.TrimSpace(r.URL.Query().Get("src"))
		if src == "" {
			writeProblem(w, r, http.StatusBadRequest, "STREAM_REQUIRED", "Managed stream name is required")
			return
		}
		found := false
		if cams, err := s.app.Cameras.List(r.Context(), 500); err == nil {
			for _, c := range cams {
				if go2rtc.CanonicalStreamName(c.ID, cameras.RoleMain) == src || go2rtc.CanonicalStreamName(c.ID, cameras.RoleDetect) == src {
					found = true
					break
				}
			}
		}
		if !found {
			writeProblem(w, r, http.StatusNotFound, "STREAM_NOT_FOUND", "Stream is not managed by Sentinel")
			return
		}
	}
	s.go2rtcProxy.ServeHTTP(w, r)
}

func validateGo2RTCURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid go2rtc URL")
	}
	return u, nil
}
