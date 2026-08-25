package httpserver

import (
	"errors"
	"net/http"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/intercom"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type intercomPutRequest struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Location     string                `json:"location,omitempty"`
	CameraID     string                `json:"camera_id,omitempty"`
	Capabilities intercom.Capabilities `json:"capabilities"`
}

type intercomUnlockRequest struct {
	Confirm       bool   `json:"confirm"`
	CorrelationID string `json:"correlation_id,omitempty"`
	TTLSeconds    int    `json:"ttl_seconds,omitempty"`
}

func (s *Server) intercomList(w http.ResponseWriter, r *http.Request) {
	if s.app.Intercom == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "INTERCOM_UNAVAILABLE", "Intercom service is unavailable")
		return
	}
	items, err := s.app.Intercom.List(r.Context(), 100)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERCOM_LIST_FAILED", "Unable to list intercom devices")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) intercomGet(w http.ResponseWriter, r *http.Request) {
	dev, err := s.app.Intercom.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "INTERCOM_NOT_FOUND", "Intercom device not found")
		return
	}
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERCOM_READ_FAILED", "Unable to read intercom device")
		return
	}
	state, err := s.app.Intercom.State(r.Context(), dev.ID)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERCOM_STATE_FAILED", "Unable to read intercom observed state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"device": dev, "observed": state})
}

func (s *Server) intercomPut(w http.ResponseWriter, r *http.Request) {
	var in intercomPutRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	if in.ID == "" {
		id, err := domain.NewID("int")
		if err != nil {
			writeProblem(w, r, http.StatusInternalServerError, "ID_GENERATION_FAILED", "Unable to allocate intercom id")
			return
		}
		in.ID = id.String()
	}
	dev, err := s.app.Intercom.Put(r.Context(), intercom.Device{ID: in.ID, Name: in.Name, Location: in.Location, CameraID: in.CameraID, Capabilities: in.Capabilities})
	if err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "INTERCOM_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

func (s *Server) intercomUnlock(w http.ResponseWriter, r *http.Request) {
	var in intercomUnlockRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	if !in.Confirm {
		writeProblem(w, r, http.StatusPreconditionRequired, "UNLOCK_CONFIRMATION_REQUIRED", "Explicit unlock confirmation is required")
		return
	}
	corr := domain.ID(in.CorrelationID)
	if !corr.ValidFor("cor") {
		id, err := domain.NewID("cor")
		if err != nil {
			writeProblem(w, r, http.StatusInternalServerError, "ID_GENERATION_FAILED", "Unable to allocate correlation id")
			return
		}
		corr = id
	}
	p, _ := principalFrom(r.Context())
	ttl := time.Duration(in.TTLSeconds) * time.Second
	record, err := s.app.Intercom.Unlock(r.Context(), intercom.UnlockRequest{DeviceID: r.PathValue("id"), ActorID: p.User.ID, CorrelationID: corr.String(), TTL: ttl})
	if err != nil {
		writeProblem(w, r, http.StatusBadGateway, "UNLOCK_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, record)
}

func (s *Server) intercomCommand(w http.ResponseWriter, r *http.Request) {
	record, err := s.app.Intercom.Commands.Get(r.Context(), r.PathValue("request_id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "INTERCOM_COMMAND_NOT_FOUND", "Intercom command not found")
		return
	}
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERCOM_COMMAND_READ_FAILED", "Unable to read intercom command")
		return
	}
	writeJSON(w, http.StatusOK, record)
}
