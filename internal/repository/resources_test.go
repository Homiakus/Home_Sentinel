//go:build sqlite_cgo

package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/database"
)

type sample struct {
	Name string `json:"name"`
}

func TestResourceStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "repo.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := NewStore[sample](db, KindCamera)
	a, err := s.Put(ctx, "cam_1", sample{Name: "Front"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Revision != 1 {
		t.Fatalf("rev=%d", a.Revision)
	}
	b, err := s.Put(ctx, "cam_1", sample{Name: "Entrance"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Revision != 2 {
		t.Fatalf("rev=%d", b.Revision)
	}
	got, err := s.Get(ctx, "cam_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value.Name != "Entrance" {
		t.Fatalf("value=%+v", got.Value)
	}
	list, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}
	if err := s.Delete(ctx, "cam_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "cam_1"); err != ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
func TestRevisionStore(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "rev.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := NewRevisionStore(db)
	r, err := s.Create(ctx, "admin", "test", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == 0 || len(r.Checksum) != 64 {
		t.Fatalf("revision=%+v", r)
	}
	latest, err := s.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != r.ID {
		t.Fatalf("latest=%d", latest.ID)
	}
}
