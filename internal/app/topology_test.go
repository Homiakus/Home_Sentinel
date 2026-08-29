package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/topology"
)

func topologyTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestOpenEnforcesSingleWriterAndReleasesOnClose(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(root, "sentinel.db")

	first, err := Open(context.Background(), cfg, topologyTestLogger())
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := Open(context.Background(), cfg, topologyTestLogger()); !errors.Is(err, topology.ErrWriterUnavailable) {
		_ = first.Close()
		t.Fatalf("second Open error=%v want %v", err, topology.ErrWriterUnavailable)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	second, err := Open(context.Background(), cfg, topologyTestLogger())
	if err != nil {
		t.Fatalf("Open after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestOpenFailureAfterWriterAcquisitionReleasesOwnership(t *testing.T) {
	runtimeRoot := t.TempDir()
	badDatabasePath := filepath.Join(runtimeRoot, "database-is-a-directory")
	if err := os.MkdirAll(badDatabasePath, 0o750); err != nil {
		t.Fatalf("create bad database path: %v", err)
	}
	cfg := config.Default()
	cfg.Database.Path = badDatabasePath

	if _, err := Open(context.Background(), cfg, topologyTestLogger()); err == nil {
		t.Fatal("Open unexpectedly succeeded with directory as sqlite database path")
	}
	guard, err := topology.AcquireWriter(runtimeRoot)
	if err != nil {
		t.Fatalf("writer ownership leaked after failed Open: %v", err)
	}
	if err := guard.Close(); err != nil {
		t.Fatalf("close recovered writer guard: %v", err)
	}
}
