//go:build sqlite_cgo

package telegram

import (
	"context"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func openStores(t *testing.T) (context.Context, PairingStore, ActionStore) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC) }
	return ctx, PairingStore{DB: db, Now: now}, ActionStore{DB: db, Now: now}
}
func TestPairingSingleUseAndActionBoundToTelegramUser(t *testing.T) {
	ctx, pairs, actions := openStores(t)
	code, _, err := pairs.Create(ctx, "usr_abc", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pairs.Consume(ctx, code, 42, 42)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pairs.Consume(ctx, code, 43, 43); err == nil {
		t.Fatal("pairing replay accepted")
	}
	corr, _ := domain.NewID("cor")
	a, err := actions.Create(ctx, b, "door.unlock", "front", corr, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := actions.Consume(ctx, 43, a.Token); err == nil {
		t.Fatal("cross-user action accepted")
	}
	got, err := actions.Consume(ctx, 42, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "front" {
		t.Fatalf("target=%s", got.Target)
	}
	if _, err := actions.Consume(ctx, 42, a.Token); err == nil {
		t.Fatal("action replay accepted")
	}
}
