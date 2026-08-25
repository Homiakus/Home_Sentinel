package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/authz"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

const sessionCookie = "sentinel_session"

type principalKeyType string

const principalKey principalKeyType = "principal"

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := domain.NewID("req")
		if err != nil {
			http.Error(w, "request id generation failed", 500)
			return
		}
		w.Header().Set("X-Request-ID", id.String())
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id.String())))
	})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(self), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.app.Sessions == nil {
			writeProblem(w, r, 503, "AUTH_NOT_READY", "Authentication storage is not ready")
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || strings.TrimSpace(c.Value) == "" {
			writeProblem(w, r, 401, "AUTH_REQUIRED", "Authentication required")
			return
		}
		p, err := s.app.Sessions.Resolve(r.Context(), c.Value)
		if err != nil {
			writeProblem(w, r, 401, "INVALID_SESSION", "Session is invalid or expired")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	})
}
func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalKey).(auth.Principal)
	return p, ok
}
func (s *Server) csrfRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			writeProblem(w, r, 401, "AUTH_REQUIRED", "Authentication required")
			return
		}
		token := r.Header.Get("X-CSRF-Token")
		if token == "" || !s.app.Sessions.ValidateCSRF(r.Context(), p.SessionID, token) {
			writeProblem(w, r, 403, "CSRF_INVALID", "CSRF token missing or invalid")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireCapability(cap authz.Capability, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok || !authz.Allowed(p.User.Role, cap) {
			writeProblem(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) freshAuthentication(maxAge time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			writeProblem(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
			return
		}
		if maxAge <= 0 {
			maxAge = 15 * time.Minute
		}
		if p.ReauthenticatedAt.IsZero() || time.Since(p.ReauthenticatedAt) > maxAge {
			writeProblem(w, r, http.StatusForbidden, "REAUTH_REQUIRED", "Recent password verification is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
