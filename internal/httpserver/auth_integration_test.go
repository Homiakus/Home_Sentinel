//go:build sqlite_cgo

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/cameras/rtsp"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/media"
)

func TestSetupLoginMeLogout(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "web.db")
	cfg.Security.SecureCookie = false
	a, err := app.Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s := New(a)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/admin", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("setup=%d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/setup/admin", bytes.NewBufferString(`{"username":"admin2","password":"correct horse battery staple"}`))
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("second setup=%d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login=%d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session cookie")
	}
	var login map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	csrf, _ := login["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("missing csrf")
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("me=%d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("logout without csrf=%d", w.Code)
	}

	// Reloading the embedded UI rotates CSRF so a token left in sessionStorage
	// cannot remain valid for the entire session lifetime.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("csrf rotate=%d %s", w.Code, w.Body.String())
	}
	var rotated map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	newCSRF, _ := rotated["csrf_token"].(string)
	if newCSRF == "" || newCSRF == csrf {
		t.Fatal("CSRF token was not rotated")
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", csrf)
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("old csrf remained valid: %d", w.Code)
	}
	csrf = newCSRF
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", csrf)
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("logout=%d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("revoked me=%d", w.Code)
	}
}

func TestCameraOnboardingAPI(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "camera-api.db")
	cfg.Security.SecureCookie = false
	a, err := app.Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Cameras.RTSPProbe = func(_ context.Context, _ string, _ time.Duration) (rtsp.Result, error) {
		return rtsp.Result{Reachable: true, Media: media.ProbeResult{Video: []media.VideoStream{{Codec: "h264", Width: 1280, Height: 720, FPS: 10}}, ProbeLatency: 10 * time.Millisecond}}, nil
	}
	s := New(a)
	setup := httptest.NewRequest(http.MethodPost, "/api/v1/setup/admin", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
	w := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, setup)
	if w.Code != 201 {
		t.Fatalf("setup=%d %s", w.Code, w.Body.String())
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, loginReq)
	if w.Code != 200 {
		t.Fatalf("login=%d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	csrf := payload["csrf_token"].(string)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cameras/onboard/rtsp", bytes.NewBufferString(`{"name":"Front","url":"rtsp://192.168.30.20/live"}`))
	req.AddCookie(cookie)
	req.Header.Set("X-CSRF-Token", csrf)
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("onboard=%d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/cameras", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"Front"`)) {
		t.Fatalf("list=%d %s", w.Code, w.Body.String())
	}
}
