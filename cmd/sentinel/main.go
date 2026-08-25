package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/backup"
	"github.com/Homiakus/Home_Sentinel/internal/buildinfo"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/httpserver"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sentinel:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sentinel <serve|version|checkpoint|restore-checkpoint>")
	}
	switch args[0] {
	case "version":
		fmt.Printf("Home Sentinel %s commit=%s built=%s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime)
		return nil
	case "serve":
		return serve()
	case "checkpoint":
		return checkpoint(args[1:])
	case "restore-checkpoint":
		return restoreCheckpoint(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	a, err := app.Open(context.Background(), cfg, log)
	if err != nil {
		return err
	}
	defer a.Close()
	s := httpserver.New(a)
	errCh := make(chan error, 1)
	go func() { log.Info("sentinel starting", "listen", cfg.Server.ListenAddress); errCh <- s.ListenAndServe() }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	select {
	case sig := <-sigCh:
		log.Info("shutdown requested", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownGrace)
		defer cancel()
		return s.Shutdown(ctx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

var _ = time.Second

func checkpoint(args []string) error {
	if len(args) != 2 || args[0] != "--out" {
		return errors.New("usage: sentinel checkpoint --out <directory>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: cfg.Database.Path, BusyTimeout: cfg.Database.BusyTimeout})
	if err != nil {
		return err
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		return err
	}
	files := append([]string(nil), cfg.Backup.ConfigFiles...)
	if source := os.Getenv("SENTINEL_CONFIG"); source != "" {
		files = append(files, source)
	}
	secretRoot := filepath.Join(filepath.Dir(cfg.Database.Path), "secrets")
	return backup.ExportCriticalToDirectory(ctx, db, files, secretRoot, args[1])
}
func restoreCheckpoint(args []string) error {
	if len(args) != 2 || args[0] != "--from" {
		return errors.New("usage: sentinel restore-checkpoint --from <directory>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	secretRoot := filepath.Join(filepath.Dir(cfg.Database.Path), "secrets")
	return backup.RestoreCriticalFromDirectory(args[1], cfg.Database.Path, secretRoot)
}
