package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/health"
	"github.com/Homiakus/Home_Sentinel/internal/integrations/go2rtc"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

func (s *Server) dashboardOverview(w http.ResponseWriter, r *http.Request) {
	cams, _ := s.app.Cameras.List(r.Context(), 500)
	incidentsOut := []any{}
	if s.app.Incidents != nil {
		items, _ := s.app.Incidents.List(r.Context(), 8)
		for _, item := range items {
			incidentsOut = append(incidentsOut, item)
		}
	}
	intercoms := []any{}
	if s.app.Intercom != nil {
		devices, _ := s.app.Intercom.List(r.Context(), 50)
		for _, d := range devices {
			state, _ := s.app.Intercom.State(r.Context(), d.ID)
			intercoms = append(intercoms, map[string]any{"device": d, "observed": state})
		}
	}
	cameraHealthy := 0
	for _, c := range cams {
		if strings.EqualFold(c.Observed.Status, "HEALTHY") {
			cameraHealthy++
		}
	}
	var backupLatest any
	if s.app.Backup != nil && s.app.Backup.Jobs != nil {
		if jobs, err := s.app.Backup.Jobs.List(r.Context(), 1); err == nil && len(jobs) > 0 {
			backupLatest = jobs[0]
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"health":    map[string]any{"status": s.app.Health.Overall(), "components": health.Diagnose(s.app.Health, dependencyGraph)},
		"cameras":   map[string]any{"total": len(cams), "healthy": cameraHealthy, "items": cams},
		"incidents": incidentsOut,
		"intercoms": intercoms,
		"hardware":  map[string]any{"memory": s.app.Hardware.Memory, "storage": s.app.Hardware.Storage, "recommendation": s.app.HardwareRecommendation},
		"backup":    map[string]any{"enabled": s.app.Backup != nil, "latest": backupLatest},
		"features":  map[string]any{"frigate": s.app.Frigate != nil, "home_assistant": s.app.HomeAssistant != nil, "ai": s.app.AI != nil, "telegram": s.app.Telegram != nil},
	})
}

func (s *Server) eventFeed(w http.ResponseWriter, r *http.Request) {
	if s.app.Incidents == nil {
		writeProblem(w, r, 503, "EVENTS_UNAVAILABLE", "Event store unavailable")
		return
	}
	limit := 100
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	items, err := s.app.Incidents.Events.List(r.Context(), 500)
	if err != nil {
		writeProblem(w, r, 500, "EVENT_LIST_FAILED", "Unable to list events")
		return
	}
	cameraFilter := strings.TrimSpace(r.URL.Query().Get("camera"))
	typeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	severityFilter := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("severity")))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := make([]any, 0, limit)
	for _, item := range items {
		ev := item.Value
		if typeFilter != "" && !strings.Contains(strings.ToLower(ev.Type), typeFilter) {
			continue
		}
		if severityFilter != "" && string(ev.Severity) != severityFilter {
			continue
		}
		if cameraFilter != "" && !payloadContains(ev.Payload, cameraFilter) {
			continue
		}
		if q != "" {
			hay := strings.ToLower(ev.Type + " " + ev.Source + " " + string(ev.Payload))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func payloadContains(raw json.RawMessage, needle string) bool {
	return strings.Contains(strings.ToLower(string(raw)), strings.ToLower(needle))
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	if s.app.Search == nil {
		writeProblem(w, r, 503, "SEARCH_UNAVAILABLE", "Search service unavailable")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 256 {
		writeProblem(w, r, 400, "SEARCH_QUERY_TOO_LONG", "Search query is too long")
		return
	}
	items, err := s.app.Search.Query(r.Context(), q, 50)
	if err != nil {
		writeProblem(w, r, 500, "SEARCH_FAILED", "Unable to search local metadata")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "mode": "metadata", "items": items, "semantic_available": false})
}

func (s *Server) cameraLiveDescriptor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, err := s.app.Cameras.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeProblem(w, r, 404, "CAMERA_NOT_FOUND", "Camera not found")
		return
	}
	if err != nil {
		writeProblem(w, r, 500, "CAMERA_READ_FAILED", "Unable to read camera")
		return
	}
	streamName := go2rtc.CanonicalStreamName(cam.ID, cameras.RoleMain)
	mode := "snapshot"
	viewer := ""
	talkURL := ""
	if s.go2rtcProxy != nil {
		mode = "go2rtc"
		viewer = "/stream.html?src=" + url.QueryEscape(streamName) + "&mode=webrtc,mse,hls,mjpeg"
		if cam.Capabilities.Talk {
			talkURL = "/webrtc.html?src=" + url.QueryEscape(streamName) + "&media=video+audio+microphone"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id": cam.ID, "name": cam.Name, "mode": mode, "stream_name": streamName,
		"viewer_url": viewer, "talk_url": talkURL, "fallback_image_url": "/api/v1/media/cameras/" + url.PathEscape(cam.ID) + "/latest.jpg",
		"talk": cam.Capabilities.Talk,
	})
}

func (s *Server) cameraDiagnostics(w http.ResponseWriter, r *http.Request) {
	cam, err := s.app.Cameras.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeProblem(w, r, 404, "CAMERA_NOT_FOUND", "Camera not found")
		return
	}
	if err != nil {
		writeProblem(w, r, 500, "CAMERA_READ_FAILED", "Unable to read camera")
		return
	}
	type check struct {
		Name, Status, Detail string
		DurationMS           int64
	}
	checks := []check{{Name: "database", Status: "OK", Detail: "camera configuration loaded"}}
	for _, st := range cam.Streams {
		if st.Endpoint.URL == "" {
			continue
		}
		start := time.Now()
		probe, probeErr := s.app.Cameras.ProbeRTSP(r.Context(), st.Endpoint)
		status, detail := "OK", "stream decoded"
		if probeErr != nil {
			status, detail = "FAILED", "media probe failed"
		} else if len(probe.Probe.Media.Video) > 0 {
			v := probe.Probe.Media.Video[0]
			detail = fmt.Sprintf("%s %dx%d %.1ffps", v.Codec, v.Width, v.Height, v.FPS)
		}
		checks = append(checks, check{Name: "stream:" + string(st.Role), Status: status, Detail: detail, DurationMS: time.Since(start).Milliseconds()})
	}
	if s.app.Frigate != nil {
		status, detail := "OK", "Frigate API reachable"
		if _, _, err := s.app.Frigate.Capabilities(r.Context()); err != nil {
			status, detail = "FAILED", "Frigate unavailable"
		}
		checks = append(checks, check{Name: "frigate", Status: status, Detail: detail})

		streamStatus, streamDetail := "FAILED", "managed stream not visible in go2rtc"
		if streams, err := s.app.Frigate.Client.Go2RTCStreams(r.Context()); err == nil {
			name := go2rtc.CanonicalStreamName(cam.ID, cameras.RoleMain)
			if _, ok := streams[name]; ok {
				streamStatus, streamDetail = "OK", "managed stream available"
			}
		}
		checks = append(checks, check{Name: "go2rtc", Status: streamStatus, Detail: streamDetail})
	}
	writeJSON(w, http.StatusOK, map[string]any{"camera": cam, "checks": checks})
}

func (s *Server) systemDiagnostics(w http.ResponseWriter, r *http.Request) {
	components := health.Diagnose(s.app.Health, dependencyGraph)
	root := make([]health.Diagnosis, 0)
	for _, d := range components {
		if d.RootCause {
			root = append(root, d)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": s.app.Health.Overall(), "components": components, "root_causes": root,
		"hardware": s.app.Hardware, "recommendation": s.app.HardwareRecommendation,
		"camera_networks": s.app.Config.Network.CameraCIDRs,
	})
}
