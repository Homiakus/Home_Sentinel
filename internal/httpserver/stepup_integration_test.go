//go:build sqlite_cgo

package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/config"
)

func TestReauthenticationRefreshesSensitiveWindow(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "db.sqlite")
	cfg.Security.SecureCookie = false
	a, err := app.Open(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	u, err := a.Users.Create(context.Background(), "admin", "correct horse battery staple", "admin")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := a.Sessions.Create(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Force the step-up timestamp stale.
	if _, err := a.DB.Exec(`UPDATE sessions SET reauthenticated_at=? WHERE id=?`, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), sess.ID); err != nil {
		t.Fatal(err)
	}
	srv := New(a)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reauth", strings.NewReader(`{"password":"correct horse battery staple"}`))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sess.Token})
	req.Header.Set("X-CSRF-Token", sess.CSRF)
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("reauth code=%d body=%s", rr.Code, rr.Body.String())
	}
	p, err := a.Sessions.Resolve(context.Background(), sess.Token)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(p.ReauthenticatedAt) > time.Minute {
		t.Fatalf("reauth timestamp not refreshed: %v", p.ReauthenticatedAt)
	}
}
