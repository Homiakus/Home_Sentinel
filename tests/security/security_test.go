package security_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/httpserver"
	"github.com/Homiakus/Home_Sentinel/internal/media"
	"github.com/Homiakus/Home_Sentinel/internal/security/netpolicy"
	"github.com/Homiakus/Home_Sentinel/internal/security/redact"
	tg "github.com/Homiakus/Home_Sentinel/internal/telegram"
)

func TestSSRFRejectsLoopbackAndLinkLocal(t *testing.T) {
	g, e := netpolicy.New([]string{"127.0.0.0/8", "169.254.0.0/16", "10.20.0.0/16"})
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range []string{"127.0.0.1", "169.254.169.254"} {
		if _, e := g.ResolveAllowed(context.Background(), h); e == nil {
			t.Fatalf("%s must be blocked", h)
		}
	}
	if _, e := g.ResolveAllowed(context.Background(), "10.20.1.2"); e != nil {
		t.Fatal(e)
	}
}
func TestRedactionRemovesSecrets(t *testing.T) {
	in := "Authorization: BearerABC password=hunter2 rtsp://admin:secret@10.0.0.2/live?token=abc"
	out := redact.String(in)
	for _, secret := range []string{"BearerABC", "hunter2", "secret", "token=abc"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret leaked in %q", out)
		}
	}
}

func TestMediaProbeDoesNotUseShell(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	argsFile := filepath.Join(dir, "args")
	script := filepath.Join(dir, "ffprobe")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsFile + "'\nprintf '%s' '{\"streams\":[{\"index\":0,\"codec_name\":\"h264\",\"codec_type\":\"video\",\"width\":640,\"height\":360,\"avg_frame_rate\":\"5/1\"}],\"format\":{\"format_name\":\"rtsp\"}}'\n"
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "ffprobe.bat")
		body = "@echo off\r\n(for %%a in (%*) do echo %%~a) > \"" + argsFile + "\"\r\necho {\"streams\":[{\"index\":0,\"codec_name\":\"h264\",\"codec_type\":\"video\",\"width\":640,\"height\":360,\"avg_frame_rate\":\"5/1\"}],\"format\":{\"format_name\":\"rtsp\"}}\r\n"
	}
	if e := os.WriteFile(script, []byte(body), 0755); e != nil {
		t.Fatal(e)
	}
	old := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+old)
	input := "rtsp://10.0.0.2/live;touch " + marker
	if _, e := media.Probe(context.Background(), input, 5*time.Second); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("shell injection executed")
	}
	b, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(b), input) {
		t.Fatalf("input was not preserved as one argument: %s", b)
	}
}

func TestTelegramActionCannotReplay(t *testing.T) {
	ctx := context.Background()
	db, e := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "s.db"), BusyTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = database.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	id, e := domain.NewID("cor")
	if e != nil {
		t.Fatal(e)
	}
	st := tg.ActionStore{DB: db}
	a, e := st.Create(ctx, tg.Binding{TelegramUserID: 42, UserID: "user1"}, "unlock", "front", id, time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = st.Consume(ctx, 42, a.Token); e != nil {
		t.Fatal(e)
	}
	if _, e = st.Consume(ctx, 42, a.Token); e == nil {
		t.Fatal("replay unexpectedly accepted")
	}
}

func TestHTTPAuthCSRFBodyLimitAndManagedStreamBoundary(t *testing.T) {
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("unmanaged stream reached go2rtc upstream") }))
	defer upstream.Close()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "s.db")
	cfg.Security.SecureCookie = false
	cfg.Frigate.Go2RTCURL = upstream.URL
	cfg.Network.CameraCIDRs = []string{"10.0.0.0/8"}
	a, e := app.Open(ctx, cfg, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	srv := httptest.NewServer(httpserver.New(a).Handler())
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/users", nil)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth bypass status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	postJSON := func(path string, v any, cookie *http.Cookie, csrf string) (*http.Response, map[string]any) {
		b, _ := json.Marshal(v)
		r, _ := http.NewRequest("POST", srv.URL+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		x, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]any
		_ = json.NewDecoder(x.Body).Decode(&m)
		x.Body.Close()
		return x, m
	}
	if r, _ := postJSON("/api/v1/setup/admin", map[string]string{"username": "admin", "password": "correct-horse-battery"}, nil, ""); r.StatusCode != 201 {
		t.Fatalf("setup=%d", r.StatusCode)
	}
	b, _ := json.Marshal(map[string]string{"username": "admin", "password": "correct-horse-battery"})
	lr, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/login", bytes.NewReader(b))
	lr.Header.Set("Content-Type", "application/json")
	lresp, e := http.DefaultClient.Do(lr)
	if e != nil {
		t.Fatal(e)
	}
	var login map[string]any
	_ = json.NewDecoder(lresp.Body).Decode(&login)
	lresp.Body.Close()
	var cookie *http.Cookie
	for _, c := range lresp.Cookies() {
		if c.Name == "sentinel_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("session cookie missing")
	}
	if r, _ := postJSON("/api/v1/users", map[string]string{"username": "x", "password": "abcdefghijkl", "role": "viewer"}, cookie, ""); r.StatusCode != http.StatusForbidden {
		t.Fatalf("CSRF bypass status=%d", r.StatusCode)
	}
	gr, _ := http.NewRequest("GET", srv.URL+"/stream.html?src=not-managed", nil)
	gr.AddCookie(cookie)
	gres, e := http.DefaultClient.Do(gr)
	if e != nil {
		t.Fatal(e)
	}
	if gres.StatusCode != http.StatusNotFound {
		bb, _ := io.ReadAll(gres.Body)
		t.Fatalf("unmanaged stream status=%d body=%s", gres.StatusCode, bb)
	}
	gres.Body.Close()
	huge := strings.Repeat("x", 70<<10)
	hr, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/login", strings.NewReader(`{"username":"`+huge+`","password":"x"}`))
	hr.Header.Set("Content-Type", "application/json")
	hresp, e := http.DefaultClient.Do(hr)
	if e != nil {
		t.Fatal(e)
	}
	if hresp.StatusCode == http.StatusOK {
		t.Fatal("oversized body accepted")
	}
	hresp.Body.Close()
}

func TestMosquittoACLDoesNotGiveGenericIntercomWildcard(t *testing.T) {
	b, e := os.ReadFile(filepath.Join("..", "..", "deploy", "mosquitto", "acl.conf"))
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	if strings.Contains(s, "user intercom\ntopic readwrite sentinel/#") {
		t.Fatal("generic intercom wildcard ACL found")
	}
	if !strings.Contains(s, "user sentinel") || !strings.Contains(s, "topic read frigate/#") {
		t.Fatal("expected least-privilege baseline missing")
	}
}
