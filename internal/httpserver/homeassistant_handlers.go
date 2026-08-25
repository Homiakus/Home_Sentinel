package httpserver

import (
	"errors"
	"net/http"
	"time"

	ha "github.com/Homiakus/Home_Sentinel/internal/integrations/homeassistant"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type homeAssistantProbeRequest struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func (s *Server) homeAssistantStatus(w http.ResponseWriter, r *http.Request) {
	if s.app.HomeAssistant == nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "reachable": false})
		return
	}
	status := s.app.HomeAssistant.Status(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "status": status})
}

func (s *Server) homeAssistantSetupGet(w http.ResponseWriter, r *http.Request) {
	if s.app.HomeAssistantSetup == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "HA_SETUP_UNAVAILABLE", "Home Assistant setup is unavailable")
		return
	}
	desired, err := s.app.HomeAssistantSetup.Get(r.Context())
	if errors.Is(err, repository.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "HA_SETUP_READ_FAILED", "Unable to read Home Assistant setup state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":       desired.Enabled,
		"url":              desired.URL,
		"verified_version": desired.VerifiedVersion,
		"configured_at":    desired.ConfiguredAt,
		"token_configured": desired.TokenRef != "",
	})
}

func (s *Server) homeAssistantProbe(w http.ResponseWriter, r *http.Request) {
	if s.app.HomeAssistantSetup == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "HA_SETUP_UNAVAILABLE", "Home Assistant setup is unavailable")
		return
	}
	var in homeAssistantProbeRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	diag := s.app.HomeAssistantSetup.Probe(r.Context(), in.URL, in.Token)
	status := http.StatusOK
	if !diag.OK {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, diag)
}

func (s *Server) homeAssistantConfigure(w http.ResponseWriter, r *http.Request) {
	var in homeAssistantProbeRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	desired, diag, err := s.app.ConfigureHomeAssistant(r.Context(), in.URL, in.Token)
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "HA_CONFIGURE_FAILED", err.Error())
		return
	}
	if !diag.OK {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"diagnostic": diag})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":       true,
		"url":              desired.URL,
		"verified_version": desired.VerifiedVersion,
		"configured_at":    desired.ConfiguredAt,
		"token_configured": true,
		"diagnostic":       diag,
	})
}

func (s *Server) homeAssistantVerifyMQTT(w http.ResponseWriter, r *http.Request) {
	if s.app.HomeAssistant == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "HA_DISABLED", "Home Assistant integration is not configured")
		return
	}
	result, err := ha.CheckMQTTDiscovery(r.Context(), s.app.HomeAssistant.REST, s.app.HomeAssistant.Discovery, 8*time.Second)
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "HA_MQTT_CHECK_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) homeAssistantVerifyFrigate(w http.ResponseWriter, r *http.Request) {
	if s.app.HomeAssistant == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "HA_DISABLED", "Home Assistant integration is not configured")
		return
	}
	result, err := ha.VerifyFrigateIntegration(r.Context(), s.app.HomeAssistant.REST, nil)
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "HA_FRIGATE_CHECK_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
