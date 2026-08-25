package httpserver

import (
	"net/http"

	"github.com/Homiakus/Home_Sentinel/internal/ai"
)

func (s *Server) aiStatus(w http.ResponseWriter, r *http.Request) {
	profile := ai.Recommend(s.app.Hardware)
	if s.app.AI == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "recommended_profile": profile})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "health": s.app.AI.Provider.Health(r.Context()), "runtime_profile": s.app.AI.Profile, "queue_depth": s.app.AI.QueueDepth()})
}
func (s *Server) aiModels(w http.ResponseWriter, r *http.Request) {
	if s.app.AI == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "AI_DISABLED", "Local AI is not enabled")
		return
	}
	models, err := s.app.AI.Provider.Models(r.Context())
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "AI_MODELS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": models})
}
func (s *Server) aiPolicyGet(w http.ResponseWriter, r *http.Request) {
	cameraID := r.PathValue("id")
	if _, err := s.app.Cameras.Get(r.Context(), cameraID); err != nil {
		writeProblem(w, r, http.StatusNotFound, "CAMERA_NOT_FOUND", "Camera not found")
		return
	}
	p, err := s.app.AIPolicies.Get(r.Context(), cameraID)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "AI_POLICY_READ_FAILED", "Unable to read AI policy")
		return
	}
	writeJSON(w, http.StatusOK, p)
}
func (s *Server) aiPolicyPut(w http.ResponseWriter, r *http.Request) {
	cameraID := r.PathValue("id")
	if _, err := s.app.Cameras.Get(r.Context(), cameraID); err != nil {
		writeProblem(w, r, http.StatusNotFound, "CAMERA_NOT_FOUND", "Camera not found")
		return
	}
	var p ai.PrivacyPolicy
	if err := decodeBody(w, r, &p); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	saved, err := s.app.AIPolicies.Put(r.Context(), cameraID, p)
	if err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "AI_POLICY_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}
