//go:build sqlite_cgo

package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/config"
)

func TestDashboardAndManagedGo2RTCProxy(t *testing.T) {
	var mu sync.Mutex
	var seenCookie, seenAuthorization, seenPath string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenCookie = r.Header.Get("Cookie")
		seenAuthorization = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		mu.Unlock()
		w.Header().Set("Set-Cookie", "go2rtc_should_not_escape=1")
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html>managed stream</html>")
	}))
	defer target.Close()

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "dashboard.db")
	cfg.Security.SecureCookie = false
	cfg.Frigate.Go2RTCURL = target.URL
	a, err := app.Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	cam := cameras.Camera{
		ID: "cam_test", Name: "Front", Type: cameras.TypeRTSP,
		Streams:  []cameras.Stream{{ID: "str_main", Role: cameras.RoleMain, Endpoint: cameras.Endpoint{URL: "rtsp://192.168.30.20/live"}, Codec: "h264", Width: 1280, Height: 720, FPS: 10}},
		Observed: cameras.Health{Status: "HEALTHY"},
	}
	if err := cam.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Cameras.Store.Put(context.Background(), cam.ID, cam); err != nil {
		t.Fatal(err)
	}
	user, err := a.Users.Create(context.Background(), "admin", "correct horse battery staple", auth.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.Sessions.Create(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}

	s := New(a)
	authed := func(method, path string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: session.Token})
		return req
	}

	w := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, authed(http.MethodGet, "/api/v1/dashboard/overview"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("overview=%d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, authed(http.MethodGet, "/api/v1/cameras/cam_test/live"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"mode":"go2rtc"`) || !strings.Contains(w.Body.String(), `stream.html`) {
		t.Fatalf("live descriptor=%d %s", w.Code, w.Body.String())
	}

	src := url.QueryEscape("cam_test")
	req := authed(http.MethodGet, "/stream.html?src="+src)
	req.Header.Set("Authorization", "Bearer sentinel-session-must-not-leak")
	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "managed stream") {
		t.Fatalf("proxy=%d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("go2rtc Set-Cookie escaped proxy: %q", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("frame policy=%q", got)
	}
	mu.Lock()
	cookie, authorization, path := seenCookie, seenAuthorization, seenPath
	mu.Unlock()
	if cookie != "" || authorization != "" {
		t.Fatalf("Sentinel credentials leaked to go2rtc: cookie=%q authorization=%q", cookie, authorization)
	}
	if path != "/stream.html" {
		t.Fatalf("go2rtc path=%q", path)
	}

	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, authed(http.MethodGet, "/api/ws?src=unmanaged"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("unmanaged ws=%d %s", w.Code, w.Body.String())
	}
}

func TestDashboardUIAssetsAreEmbedded(t *testing.T) {
	a, err := app.New(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	s := New(a)
	for _, tc := range []struct{ path, want string }{
		{"/", "Home Sentinel"},
		{"/assets/app.js", "function connectRealtime"},
		{"/assets/styles.css", ".camera-wall"},
	} {
		w := httptest.NewRecorder()
		s.http.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s=%d missing %q", tc.path, w.Code, tc.want)
		}
	}
}

func TestUserManagementPreservesLastEnabledAdmin(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "users.db")
	cfg.Security.SecureCookie = false
	a, err := app.Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	admin, err := a.Users.Create(context.Background(), "admin", "correct horse battery staple", auth.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.Sessions.Create(context.Background(), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := New(a)
	authedJSON := func(method, path, body string) *http.Request {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: session.Token})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", session.CSRF)
		return req
	}

	w := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, authedJSON(http.MethodPatch, "/api/v1/users/"+admin.ID, `{"role":"viewer","disabled":false}`))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "LAST_ADMIN") {
		t.Fatalf("last admin demotion=%d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, authedJSON(http.MethodPost, "/api/v1/users", `{"username":"backupadmin","password":"another correct battery staple","role":"admin"}`))
	if w.Code != http.StatusCreated {
		t.Fatalf("create second admin=%d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(w, authedJSON(http.MethodPatch, "/api/v1/users/"+admin.ID, `{"role":"viewer","disabled":false}`))
	if w.Code != http.StatusOK {
		t.Fatalf("demote with second admin=%d %s", w.Code, w.Body.String())
	}
}
