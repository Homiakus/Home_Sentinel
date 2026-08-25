package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	if s.app.Users == nil {
		writeProblem(w, r, 503, "SETUP_NOT_READY", "Setup storage is not ready")
		return
	}
	n, err := s.app.Users.Count(r.Context())
	if err != nil {
		writeProblem(w, r, 500, "SETUP_STATUS_FAILED", "Unable to read setup status")
		return
	}
	writeJSON(w, 200, map[string]any{"needs_admin": n == 0})
}
func (s *Server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	if s.app.Users == nil {
		writeProblem(w, r, 503, "SETUP_NOT_READY", "Setup storage is not ready")
		return
	}
	n, err := s.app.Users.Count(r.Context())
	if err != nil {
		writeProblem(w, r, 500, "SETUP_STATUS_FAILED", "Unable to read setup status")
		return
	}
	if n != 0 {
		writeProblem(w, r, 409, "SETUP_COMPLETE", "Initial administrator already exists")
		return
	}
	var in credentials
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, 400, "INVALID_JSON", "Invalid request body")
		return
	}
	u, err := s.app.Users.Create(r.Context(), in.Username, in.Password, auth.RoleAdmin)
	if err != nil {
		writeProblem(w, r, 400, "USER_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"user": u})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.app.Users == nil || s.app.Sessions == nil {
		writeProblem(w, r, 503, "AUTH_NOT_READY", "Authentication storage is not ready")
		return
	}
	var in credentials
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, 400, "INVALID_JSON", "Invalid request body")
		return
	}
	u, err := s.app.Users.Authenticate(r.Context(), in.Username, in.Password)
	if err != nil {
		writeProblem(w, r, 401, "INVALID_CREDENTIALS", "Invalid username or password")
		return
	}
	session, err := s.app.Sessions.Create(r.Context(), u.ID)
	if err != nil {
		writeProblem(w, r, 500, "SESSION_CREATE_FAILED", "Unable to create session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: session.Token, Path: "/", HttpOnly: true, Secure: s.app.Config.Security.SecureCookie, SameSite: http.SameSiteStrictMode, Expires: session.ExpiresAt})
	writeJSON(w, 200, map[string]any{"user": u, "csrf_token": session.CSRF, "expires_at": session.ExpiresAt})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	writeJSON(w, 200, map[string]any{"user": p.User, "session_expires_at": p.ExpiresAt})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.app.Sessions.Revoke(r.Context(), p.SessionID); err != nil {
		writeProblem(w, r, 500, "LOGOUT_FAILED", "Unable to revoke session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", Value: "", MaxAge: -1, HttpOnly: true, Secure: s.app.Config.Security.SecureCookie, SameSite: http.SameSiteStrictMode, Expires: time.Unix(1, 0)})
	w.WriteHeader(http.StatusNoContent)
}

type reauthRequest struct {
	Password string `json:"password"`
}

func (s *Server) reauthenticate(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}
	var in reauthRequest
	if err := decodeBody(w, r, &in); err != nil || in.Password == "" {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_REAUTH", "Password is required")
		return
	}
	if _, err := s.app.Users.Authenticate(r.Context(), p.User.Username, in.Password); err != nil {
		writeProblem(w, r, http.StatusUnauthorized, "REAUTH_FAILED", "Password verification failed")
		return
	}
	if err := s.app.Sessions.MarkReauthenticated(r.Context(), p.SessionID); err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "REAUTH_FAILED", "Unable to update session authentication state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reauthenticated": true, "valid_for_seconds": 900})
}

func (s *Server) csrf(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}
	token, err := s.app.Sessions.RotateCSRF(r.Context(), p.SessionID)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "CSRF_ROTATE_FAILED", "Unable to refresh CSRF token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"csrf_token": token})
}
