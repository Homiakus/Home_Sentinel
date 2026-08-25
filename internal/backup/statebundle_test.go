//go:build sqlite_cgo

package backup

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

func TestStateBundleRoundTrip(t *testing.T) {
	ctx := context.Background()
	src, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "src.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if err := database.Migrate(ctx, src); err != nil {
		t.Fatal(err)
	}
	store := repository.NewStore[map[string]string](src, repository.KindDevice)
	if _, err := store.Put(ctx, "dev_1", map[string]string{"name": "door"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.NewRevisionStore(src).Create(ctx, "admin", "init", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ExportState(ctx, src, &buf); err != nil {
		t.Fatal(err)
	}
	dst, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "dst.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	if err := database.Migrate(ctx, dst); err != nil {
		t.Fatal(err)
	}
	if err := ImportState(ctx, dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	got, err := repository.NewStore[map[string]string](dst, repository.KindDevice).Get(ctx, "dev_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value["name"] != "door" {
		t.Fatalf("got=%v", got.Value)
	}
}
