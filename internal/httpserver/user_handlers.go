package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type userCreateRequest struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

type userAccessRequest struct {
	Role     auth.Role `json:"role"`
	Disabled bool      `json:"disabled"`
}

func (s *Server) userList(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.Users.List(r.Context(), 200)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "USER_LIST_FAILED", "Unable to list users")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) userCreate(w http.ResponseWriter, r *http.Request) {
	var in userCreateRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	if in.Role == "" {
		in.Role = auth.RoleViewer
	}
	u, err := s.app.Users.Create(r.Context(), in.Username, in.Password, in.Role)
	if err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "USER_CREATE_FAILED", err.Error())
		return
	}
	s.auditUserChange(r, "user.create", u.ID, "success", map[string]any{"username": u.Username, "role": u.Role})
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) userAccessUpdate(w http.ResponseWriter, r *http.Request) {
	var in userAccessRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}
	u, err := s.app.Users.UpdateAccess(r.Context(), r.PathValue("id"), in.Role, in.Disabled)
	if errors.Is(err, auth.ErrLastEnabledAdmin) {
		writeProblem(w, r, http.StatusConflict, "LAST_ADMIN", "The last enabled administrator cannot be disabled or demoted")
		return
	}
	if err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "USER_UPDATE_FAILED", err.Error())
		return
	}
	s.auditUserChange(r, "user.access.update", u.ID, "success", map[string]any{"role": u.Role, "disabled": u.Disabled})
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) auditUserChange(r *http.Request, action, target, result string, details any) {
	if s.app.Audit == nil {
		return
	}
	p, _ := principalFrom(r.Context())
	body, _ := json.Marshal(details)
	_, _ = s.app.Audit.Append(r.Context(), repository.AuditEntry{OccurredAt: time.Now().UTC(), Actor: p.User.ID, Source: "web", Action: action, Target: target, Result: result, RequestID: requestIDFrom(r.Context()), Details: body})
}
