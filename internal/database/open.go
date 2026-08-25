package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Path        string
	BusyTimeout time.Duration
	MaxOpen     int
	MaxIdle     int
}

func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	if opts.Path == "" {
		return nil, errors.New("database path required")
	}
	if opts.BusyTimeout <= 0 {
		opts.BusyTimeout = 5 * time.Second
	}
	if opts.MaxOpen <= 0 {
		opts.MaxOpen = 4
	}
	if opts.MaxIdle < 0 {
		opts.MaxIdle = 0
	}
	if opts.MaxIdle == 0 {
		opts.MaxIdle = 2
	}
	if opts.Path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(opts.Path), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := buildDSN(opts.Path, opts.BusyTimeout)
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(opts.MaxOpen)
	db.SetMaxIdleConns(opts.MaxIdle)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	// Apply critical pragmas on the pool as well. DSN pragmas make them apply
	// per connection for the modernc backend; these statements verify support.
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		fmt.Sprintf("PRAGMA busy_timeout = %d", opts.BusyTimeout.Milliseconds()),
		"PRAGMA synchronous = NORMAL",
	}
	for _, q := range pragmas {
		if _, err := db.ExecContext(ctx, q); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite pragma %q: %w", q, err)
		}
	}
	return db, nil
}

func buildDSN(path string, busy time.Duration) string {
	if path == ":memory:" {
		// Shared in-memory DB is required when database/sql opens >1 connection.
		return "file:sentinel-memory?mode=memory&cache=shared"
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	uri := "file:" + clean
	if strings.HasPrefix(clean, "/") {
		uri = "file://" + clean
	}
	pragmas := url.Values{}
	pragmas.Add("_pragma", "foreign_keys(1)")
	pragmas.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busy.Milliseconds()))
	pragmas.Add("_pragma", "journal_mode(WAL)")
	pragmas.Add("_pragma", "synchronous(NORMAL)")
	return uri + "?" + pragmas.Encode()
}
