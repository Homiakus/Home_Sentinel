package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/health"
	tgapi "github.com/Homiakus/Home_Sentinel/internal/integrations/telegram"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	tgsvc "github.com/Homiakus/Home_Sentinel/internal/telegram"
)

type incidentRuntimeCallbackAuthority struct{}

func (incidentRuntimeCallbackAuthority) Accept(string, callback.Binding) (callback.Claims, error) {
	return callback.Claims{Subject: "usr_00000000000000000000000000"}, nil
}

func (incidentRuntimeCallbackAuthority) Sign(callback.Claims) (string, error) { return "signed", nil }

func TestStartIncidentRuntimeKeepsAuthorityOnlyModeTransportFree(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
	a := &App{
		Config:           cfg,
		CallbackSecurity: incidentRuntimeCallbackAuthority{},
		Health:           health.NewRegistry(),
		Log:              slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := a.startIncidentRuntime(); err != nil {
		t.Fatal(err)
	}
	if a.IncidentRuntime != nil {
		t.Fatal("incident runtime started without notification transport")
	}
	if a.IncidentCallbacks != nil {
		t.Fatal("callback ingress exposed without durable incident runtime")
	}
}

func TestStartIncidentRuntimeRejectsPartialProductionDependencies(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
	a := &App{
		Config:   cfg,
		Telegram: &tgsvc.Service{},
		Health:   health.NewRegistry(),
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := a.startIncidentRuntime(); err == nil || !strings.Contains(err.Error(), "production dependencies unavailable") {
		t.Fatalf("startIncidentRuntime() error=%v, want dependency failure", err)
	}
	if a.IncidentRuntime != nil || a.IncidentCallbacks != nil {
		t.Fatal("partial dependency failure exposed incident runtime")
	}
}

func TestIncidentRuntimeLifecycleIsDurableRestartableAndKeepsSQLiteOpen(t *testing.T) {
	a := newIncidentRuntimeTestApp(t, true)
	root := filepath.Join(filepath.Dir(a.Config.Database.Path), "orchestration", "incident")

	if err := a.startIncidentRuntime(); err != nil {
		t.Fatal(err)
	}
	if a.IncidentRuntime == nil {
		t.Fatal("durable incident runtime was not initialized")
	}
	if a.IncidentCallbacks == nil {
		t.Fatal("callback ingress was not bound to initialized runtime")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("deterministic incident runtime root %q unavailable: %v", root, err)
	}
	if component, ok := a.Health.Get("incident_runtime"); !ok || (component.Status != health.Starting && component.Status != health.Healthy) {
		t.Fatalf("incident runtime health=%+v ok=%v", component, ok)
	}
	if err := a.startIncidentRuntime(); err == nil || !strings.Contains(err.Error(), "already started") {
		t.Fatalf("second start error=%v, want already-started fence", err)
	}

	a.runCancel()
	if err := a.stopIncidentRuntime(); err != nil {
		t.Fatal(err)
	}
	if a.IncidentRuntime != nil || a.IncidentCallbacks != nil || a.incidentServeDone != nil {
		t.Fatal("incident runtime lifecycle state was not cleared after stop")
	}
	if err := a.DB.PingContext(context.Background()); err != nil {
		t.Fatalf("SQLite was closed before incident runtime shutdown completed: %v", err)
	}

	// Re-open the exact same deterministic ADGO root under a fresh application
	// lifecycle context. This proves shutdown released the durable store cleanly.
	a.runCtx, a.runCancel = context.WithCancel(context.Background())
	if err := a.startIncidentRuntime(); err != nil {
		t.Fatalf("restart durable incident runtime: %v", err)
	}
	a.runCancel()
	if err := a.stopIncidentRuntime(); err != nil {
		t.Fatalf("stop restarted incident runtime: %v", err)
	}
}

func TestIncidentRuntimeWithoutCallbackAuthorityDoesNotExposeIngress(t *testing.T) {
	a := newIncidentRuntimeTestApp(t, false)
	if err := a.startIncidentRuntime(); err != nil {
		t.Fatal(err)
	}
	if a.IncidentRuntime == nil {
		t.Fatal("runtime not initialized")
	}
	if a.IncidentCallbacks != nil {
		t.Fatal("callback ingress exposed without callback authority")
	}
	a.runCancel()
	if err := a.stopIncidentRuntime(); err != nil {
		t.Fatal(err)
	}
}

func newIncidentRuntimeTestApp(t *testing.T, callbacks bool) *App {
	t.Helper()
	ctx := context.Background()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
	db, err := database.Open(ctx, database.Options{Path: cfg.Database.Path, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	client, err := tgapi.New(tgapi.Options{Token: "test-token", BaseURL: "http://127.0.0.1:1"})
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	a := &App{
		Config: cfg,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:     db,
		Users:  auth.NewUserStore(db),
		Audit:  repository.NewAuditStore(db),
		Telegram: &tgsvc.Service{
			Client:   client,
			Pairings: tgsvc.PairingStore{DB: db},
		},
		Health:    health.NewRegistry(),
		runCtx:    runCtx,
		runCancel: cancel,
	}
	if callbacks {
		a.CallbackSecurity = incidentRuntimeCallbackAuthority{}
	}
	t.Cleanup(func() {
		if a.runCancel != nil {
			a.runCancel()
		}
		if a.IncidentRuntime != nil {
			_ = a.stopIncidentRuntime()
		}
		_ = db.Close()
	})
	return a
}
