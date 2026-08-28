package app

import (
	"context"
	"errors"
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

func TestIncidentRuntimeShutdownTimeoutContract(t *testing.T) {
	if incidentRuntimeShutdownTimeout != 10*time.Second {
		t.Fatalf("shutdown timeout=%s want 10s", incidentRuntimeShutdownTimeout)
	}
}

func TestStartIncidentRuntimeNilApplicationFails(t *testing.T) {
	var a *App
	if err := a.startIncidentRuntime(); err == nil || !strings.Contains(err.Error(), "application is required") {
		t.Fatalf("nil application error=%v", err)
	}
}

func TestStopIncidentRuntimeNoopWithoutRuntime(t *testing.T) {
	var nilApp *App
	if err := nilApp.stopIncidentRuntime(); err != nil {
		t.Fatalf("nil app stop error=%v", err)
	}
	if err := (&App{}).stopIncidentRuntime(); err != nil {
		t.Fatalf("empty app stop error=%v", err)
	}
}

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

func TestStartIncidentRuntimeRejectsEachMissingProductionDependency(t *testing.T) {
	cases := []struct {
		name string
		want string
		drop func(*App)
	}{
		{name: "telegram client", want: "Telegram client unavailable", drop: func(a *App) { a.Telegram.Client = nil }},
		{name: "database", want: "database unavailable", drop: func(a *App) { a.DB = nil }},
		{name: "user store", want: "user store unavailable", drop: func(a *App) { a.Users = nil }},
		{name: "audit store", want: "audit store unavailable", drop: func(a *App) { a.Audit = nil }},
		{name: "lifecycle context", want: "lifecycle context unavailable", drop: func(a *App) { a.runCtx = nil }},
		{name: "health registry", want: "health registry unavailable", drop: func(a *App) { a.Health = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newIncidentRuntimeTestApp(t, true)
			tc.drop(a)
			err := a.startIncidentRuntime()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("startIncidentRuntime() error=%v want containing %q", err, tc.want)
			}
			if a.IncidentRuntime != nil || a.IncidentCallbacks != nil {
				t.Fatal("missing dependency exposed incident runtime")
			}
		})
	}
}

func TestClassifyIncidentServeExitMatrix(t *testing.T) {
	serveErr := errors.New("serve failed")
	cases := []struct {
		name          string
		lifecycleErr  error
		serveErr      error
		wantUnexpected bool
		wantNil       bool
		wantServeErr  bool
	}{
		{name: "shutdown nil result", lifecycleErr: context.Canceled, serveErr: nil, wantNil: true},
		{name: "shutdown preserves result", lifecycleErr: context.Canceled, serveErr: serveErr, wantServeErr: true},
		{name: "active nil result is unexpected", wantUnexpected: true},
		{name: "active error is unexpected", serveErr: serveErr, wantUnexpected: true, wantServeErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr, unexpected := classifyIncidentServeExit(tc.lifecycleErr, tc.serveErr)
			if unexpected != tc.wantUnexpected {
				t.Fatalf("unexpected=%v want=%v error=%v", unexpected, tc.wantUnexpected, gotErr)
			}
			if tc.wantNil && gotErr != nil {
				t.Fatalf("error=%v want nil", gotErr)
			}
			if tc.wantServeErr && !errors.Is(gotErr, serveErr) {
				t.Fatalf("error=%v want serve error", gotErr)
			}
			if tc.wantUnexpected && gotErr == nil {
				t.Fatal("unexpected exit did not produce an error")
			}
		})
	}
}

func TestWaitIncidentServeFaultMatrix(t *testing.T) {
	if err := waitIncidentServe(nil, context.Canceled, time.Second); err == nil || !strings.Contains(err.Error(), "completion channel missing") {
		t.Fatalf("nil completion channel error=%v", err)
	}
	ready := make(chan error, 1)
	ready <- nil
	if err := waitIncidentServe(ready, context.Canceled, 0); err == nil || !strings.Contains(err.Error(), "timeout must be positive") {
		t.Fatalf("zero timeout error=%v", err)
	}

	timedOut := make(chan error)
	if err := waitIncidentServe(timedOut, context.Canceled, time.Millisecond); err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("timeout error=%v", err)
	}

	shutdownErr := errors.New("worker noticed cancellation")
	shutdown := make(chan error, 1)
	shutdown <- shutdownErr
	if err := waitIncidentServe(shutdown, context.Canceled, time.Second); err != nil {
		t.Fatalf("canceled lifecycle should ignore terminal serve error: %v", err)
	}

	unexpectedNil := make(chan error, 1)
	unexpectedNil <- nil
	if err := waitIncidentServe(unexpectedNil, nil, time.Second); err == nil || !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Fatalf("active lifecycle nil result error=%v", err)
	}

	serveErr := errors.New("serve failed")
	unexpectedErr := make(chan error, 1)
	unexpectedErr <- serveErr
	if err := waitIncidentServe(unexpectedErr, nil, time.Second); !errors.Is(err, serveErr) {
		t.Fatalf("active lifecycle serve error=%v want wrapped serve failure", err)
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

func TestAppCloseStopsIncidentRuntimeBeforeClosingSQLite(t *testing.T) {
	a := newIncidentRuntimeTestApp(t, true)
	db := a.DB
	if err := a.startIncidentRuntime(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if a.IncidentRuntime != nil || a.IncidentCallbacks != nil {
		t.Fatal("App.Close left incident runtime state exposed")
	}
	if a.Telegram != nil {
		t.Fatal("App.Close returned before closing Telegram dependency")
	}
	if a.DB != nil {
		t.Fatal("App.Close returned before clearing SQLite dependency")
	}
	if err := db.PingContext(context.Background()); err == nil {
		t.Fatal("SQLite remained open after App.Close")
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
