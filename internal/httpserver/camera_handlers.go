package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/realtime"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type rtspOnboardBody struct {
	Name        string             `json:"name"`
	URL         string             `json:"url"`
	Username    string             `json:"username,omitempty"`
	PasswordRef string             `json:"password_ref,omitempty"`
	Role        cameras.StreamRole `json:"role,omitempty"`
}

func (s *Server) cameraList(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	items, err := s.app.Cameras.List(r.Context(), limit)
	if err != nil {
		writeProblem(w, r, 500, "CAMERA_LIST_FAILED", "Unable to list cameras")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (s *Server) cameraGet(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	cam, err := s.app.Cameras.Get(r.Context(), r.PathValue("id"))
	if err == repository.ErrNotFound {
		writeProblem(w, r, 404, "CAMERA_NOT_FOUND", "Camera not found")
		return
	}
	if err != nil {
		writeProblem(w, r, 500, "CAMERA_READ_FAILED", "Unable to read camera")
		return
	}
	writeJSON(w, 200, cam)
}
func (s *Server) cameraOnboardRTSP(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	var in rtspOnboardBody
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, 400, "INVALID_JSON", "Invalid request body")
		return
	}
	var ref secrets.Ref
	if in.PasswordRef != "" {
		parsed, err := secrets.ParseRef(in.PasswordRef)
		if err != nil {
			writeProblem(w, r, 400, "INVALID_SECRET_REF", "Invalid password secret reference")
			return
		}
		ref = parsed
	}
	cam, err := s.app.Cameras.OnboardRTSP(r.Context(), cameras.RTSPOnboardRequest{Name: in.Name, URL: in.URL, Username: in.Username, PasswordRef: ref, Role: in.Role})
	if err != nil {
		writeProblem(w, r, 422, "CAMERA_PROBE_FAILED", "Camera stream could not be validated")
		return
	}
	p, _ := principalFrom(r.Context())
	if s.app.Audit != nil {
		details, _ := json.Marshal(map[string]any{"camera_id": cam.ID, "type": cam.Type})
		_, _ = s.app.Audit.Append(r.Context(), repository.AuditEntry{OccurredAt: time.Now().UTC(), Actor: p.User.ID, Source: "web", Action: "camera.onboard", Target: cam.ID, Result: "success", RequestID: requestIDFrom(r.Context()), Details: details})
	}
	if id, e := domain.NewID("evt"); e == nil {
		s.app.Realtime.Publish(realtime.Message{ID: id.String(), Type: "camera.added", Data: mustJSON(map[string]any{"camera_id": cam.ID})})
	}
	writeJSON(w, 201, cam)
}
func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

type onvifOnboardBody struct {
	Name        string `json:"name"`
	DeviceURL   string `json:"device_url"`
	Username    string `json:"username,omitempty"`
	PasswordRef string `json:"password_ref,omitempty"`
}

func (s *Server) cameraDiscoverONVIF(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	duration := 3 * time.Second
	if raw := r.URL.Query().Get("duration"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 && d <= 5*time.Second {
			duration = d
		}
	}
	items, err := s.app.Cameras.DiscoverONVIF(r.Context(), duration)
	if err != nil {
		writeProblem(w, r, 502, "ONVIF_DISCOVERY_FAILED", "ONVIF discovery failed")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "duration": duration.String()})
}
func (s *Server) cameraOnboardONVIF(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	var in onvifOnboardBody
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, 400, "INVALID_JSON", "Invalid request body")
		return
	}
	var ref secrets.Ref
	if in.PasswordRef != "" {
		parsed, err := secrets.ParseRef(in.PasswordRef)
		if err != nil {
			writeProblem(w, r, 400, "INVALID_SECRET_REF", "Invalid password secret reference")
			return
		}
		ref = parsed
	}
	cam, err := s.app.Cameras.OnboardONVIF(r.Context(), cameras.ONVIFOnboardRequest{Name: in.Name, DeviceURL: in.DeviceURL, Username: in.Username, PasswordRef: ref})
	if err != nil {
		writeProblem(w, r, 422, "ONVIF_ONBOARD_FAILED", "ONVIF camera could not be validated")
		return
	}
	p, _ := principalFrom(r.Context())
	if s.app.Audit != nil {
		details, _ := json.Marshal(map[string]any{"camera_id": cam.ID, "manufacturer": cam.Manufacturer, "model": cam.Model})
		_, _ = s.app.Audit.Append(r.Context(), repository.AuditEntry{OccurredAt: time.Now().UTC(), Actor: p.User.ID, Source: "web", Action: "camera.onboard", Target: cam.ID, Result: "success", RequestID: requestIDFrom(r.Context()), Details: details})
	}
	if id, e := domain.NewID("evt"); e == nil {
		s.app.Realtime.Publish(realtime.Message{ID: id.String(), Type: "camera.added", Data: mustJSON(map[string]any{"camera_id": cam.ID, "type": "onvif"})})
	}
	writeJSON(w, 201, cam)
}

type uvcOnboardBody struct {
	Name string             `json:"name"`
	Path string             `json:"path"`
	Role cameras.StreamRole `json:"role,omitempty"`
}

func (s *Server) cameraDiscoverUVC(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	items, err := s.app.Cameras.DiscoverUVC(r.Context())
	if err != nil {
		writeProblem(w, r, 500, "UVC_DISCOVERY_FAILED", "USB camera discovery failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) cameraOnboardUVC(w http.ResponseWriter, r *http.Request) {
	if s.app.Cameras == nil {
		writeProblem(w, r, 503, "CAMERAS_NOT_READY", "Camera service is not ready")
		return
	}
	var in uvcOnboardBody
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, 400, "INVALID_JSON", "Invalid request body")
		return
	}
	cam, err := s.app.Cameras.OnboardUVC(r.Context(), cameras.UVCOnboardRequest{Name: in.Name, Path: in.Path, Role: in.Role})
	if err != nil {
		writeProblem(w, r, 422, "UVC_ONBOARD_FAILED", "USB camera could not be added: "+err.Error())
		return
	}
	p, _ := principalFrom(r.Context())
	if s.app.Audit != nil {
		details, _ := json.Marshal(map[string]any{"camera_id": cam.ID, "type": cam.Type, "device": in.Path})
		_, _ = s.app.Audit.Append(r.Context(), repository.AuditEntry{OccurredAt: time.Now().UTC(), Actor: p.User.ID, Source: "web", Action: "camera.onboard", Target: cam.ID, Result: "success", RequestID: requestIDFrom(r.Context()), Details: details})
	}
	if id, e := domain.NewID("evt"); e == nil {
		s.app.Realtime.Publish(realtime.Message{ID: id.String(), Type: "camera.added", Data: mustJSON(map[string]any{"camera_id": cam.ID, "type": "uvc"})})
	}
	writeJSON(w, 201, cam)
}

