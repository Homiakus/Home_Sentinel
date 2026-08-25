//go:build sqlite_cgo

package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/database"
)

func openAuthDB(t *testing.T) (context.Context, *UserStore, *SessionStore) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "auth.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return ctx, NewUserStore(db), NewSessionStore(db, time.Hour)
}
func TestPasswordHash(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if h == "correct horse battery staple" {
		t.Fatal("plaintext password stored")
	}
	if !VerifyPassword(h, "correct horse battery staple") {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword(h, "wrong password") {
		t.Fatal("invalid password accepted")
	}
}
func TestUserAndSessionLifecycle(t *testing.T) {
	ctx, users, sessions := openAuthDB(t)
	u, err := users.Create(ctx, "admin", "correct horse battery staple", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	got, err := users.Authenticate(ctx, "ADMIN", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatalf("id=%s want=%s", got.ID, u.ID)
	}
	s, err := sessions.Create(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := sessions.Resolve(ctx, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	if p.User.ID != u.ID {
		t.Fatalf("principal=%+v", p)
	}
	if !sessions.ValidateCSRF(ctx, p.SessionID, s.CSRF) {
		t.Fatal("csrf rejected")
	}
	if err := sessions.Revoke(ctx, p.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Resolve(ctx, s.Token); err == nil {
		t.Fatal("revoked session accepted")
	}
}
