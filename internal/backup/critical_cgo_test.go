//go:build sqlite_cgo

package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/database"
)

func TestCriticalBundleSQLiteSnapshotAndTamperDetection(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "live.db"), BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_meta(key,value) VALUES('proof','wal-safe')`); err != nil {
		t.Fatal(err)
	}

	secretsDir := filepath.Join(t.TempDir(), "secrets")
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "camera"), []byte("cam-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "backup-password"), []byte("must-not-backup"), 0o600); err != nil {
		t.Fatal(err)
	}

	bundle, err := PrepareCriticalBundle(ctx, db, nil, secretsDir, []string{"backup-password"})
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Cleanup()
	if err := VerifyCriticalBundle(bundle.Root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bundle.Root, "secrets", "backup-password")); !os.IsNotExist(err) {
		t.Fatalf("backup password should be excluded, err=%v", err)
	}

	restored, err := database.Open(ctx, database.Options{Path: filepath.Join(bundle.Root, "state", "sentinel.db"), BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var value string
	if err := restored.QueryRow(`SELECT value FROM schema_meta WHERE key='proof'`).Scan(&value); err != nil {
		restored.Close()
		t.Fatal(err)
	}
	restored.Close()
	if value != "wal-safe" {
		t.Fatalf("snapshot value=%q", value)
	}

	if err := os.WriteFile(filepath.Join(bundle.Root, "state", "state-bundle.json"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCriticalBundle(bundle.Root); err == nil {
		t.Fatal("tampering must be detected")
	}
}
