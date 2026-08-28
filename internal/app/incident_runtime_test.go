package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	"github.com/Homiakus/Home_Sentinel/internal/health"
	tgapi "github.com/Homiakus/Home_Sentinel/internal/integrations/telegram"
	orchestrationincident "github.com/Homiakus/Home_Sentinel/internal/orchestration/incident"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	tgsvc "github.com/Homiakus/Home_Sentinel/internal/telegram"
	"github.com/Homiakus/axiom/adgo"
)

type appIncidentNotifierFake struct {
	mu    sync.Mutex
	calls int
}

func (f *appIncidentNotifierFake) Notify(context.Context, gateway.Operation, gateway.Notification) (gateway.EffectResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return gateway.EffectResult{State: gateway.EffectApplied, ProviderID: "test"}, nil
}

type appIncidentSecurityFake struct{}

func (appIncidentSecurityFake) Accept(string, callback.Binding) (callback.Claims, error) {
	return callback.Claims{}, errors.New("not used")
}
func (appIncidentSecurityFake) Sign(callback.Claims) (string, error) { return "signed", nil }

type appIncidentUsersFake struct{}

func (appIncidentUsersFake) GetByID(context.Context, string) (auth.User, error) {
	return auth.User{}, errors.New("not used")
}

type appIncidentAuditFake struct{}

func (appIncidentAuditFake) Append(context.Context, repository.AuditEntry) (repository.AuditEntry, error) {
	return repository.AuditEntry{}, nil
}

type appIncidentWorkflowFake struct {
	serveErr error
	closeErr error

	startResult   *adgo.Execution
	ownerResult   *adgo.Execution
	resolveResult *adgo.Execution
	startErr      error
	ownerErr      error
	resolveErr    error

	mu           sync.Mutex
	startCalls   int
	ownerCalls   int
	resolveCalls int
	serveCalls   int
	closeCalls   int
}

func (f *appIncidentWorkflowFake) Start(context.Context, domainincident.Trigger) (*adgo.Execution, error) {
	f.mu.Lock()
	f.startCalls++
	f.mu.Unlock()
	return f.startResult, f.startErr
}
func (f *appIncidentWorkflowFake) Serve(context.Context) error {
	f.mu.Lock()
	f.serveCalls++
	f.mu.Unlock()
	return f.serveErr
}
func (f *appIncidentWorkflowFake) OwnerResponse(context.Context, string, string, any) (*adgo.Execution, error) {
	f.mu.Lock()
	f.ownerCalls++
	f.mu.Unlock()
	return f.ownerResult, f.ownerErr
}
func (f *appIncidentWorkflowFake) ResolveOwnerCallbackDecision(context.Context, string, string, domainincident.Decision, string, string, any) (*adgo.Execution, error) {
	f.mu.Lock()
	f.resolveCalls++
	f.mu.Unlock()
	return f.resolveResult, f.resolveErr
}
func (f *appIncidentWorkflowFake) Close() error {
	f.mu.Lock()
	f.closeCalls++
	f.mu.Unlock()
	return f.closeErr
}

func memoryIncidentConfig() orchestrationincident.Config {
	cfg := orchestrationincident.DefaultConfig("")
	cfg.Production.Backend = adgo.BackendMemory
	cfg.Production.Root = ""
	cfg.Production.PollInterval = time.Millisecond
	cfg.Production.CoordinatorInterval = time.Millisecond
	return cfg
}

func validAppIncidentTrigger() domainincident.Trigger {
	return domainincident.Trigger{
		EventID:    "evt-runtime-1",
		SourceID:   "camera-front",
		Kind:       "person.detected",
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		Confidence: 0.2,
	}
}

func TestOpenIncidentRuntimeRequiresParentAndDurableNotifier(t *testing.T) {
	if _, err := openIncidentRuntime(nil, incidentRuntimeOptions{Config: memoryIncidentConfig(), Notifier: &appIncidentNotifierFake{}}); err == nil {
		t.Fatal("incident runtime opened without parent context")
	}
	if _, err := openIncidentRuntime(context.Background(), incidentRuntimeOptions{Config: memoryIncidentConfig()}); err == nil {
		t.Fatal("incident runtime opened without notifier")
	}
}

func TestOpenIncidentRuntimeCallbackAuthorityUsesSameRuntime(t *testing.T) {
	notifier := &appIncidentNotifierFake{}
	runtime, err := openIncidentRuntime(context.Background(), incidentRuntimeOptions{
		Config:           memoryIncidentConfig(),
		Notifier:         notifier,
		CallbacksEnabled: true,
		Security:         appIncidentSecurityFake{},
		Users:            appIncidentUsersFake{},
		Audit:            appIncidentAuditFake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Callbacks == nil {
		t.Fatal("enabled callbacks were not wired")
	}
	if runtime.Callbacks.Workflow != runtime {
		t.Fatal("callback ingress is not bound to the owning incident runtime")
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("normal runtime close: %v", err)
	}
}

func TestOpenIncidentRuntimeEnabledCallbacksFailClosedForEachMissingAuthority(t *testing.T) {
	base := incidentRuntimeOptions{
		Config:           memoryIncidentConfig(),
		Notifier:         &appIncidentNotifierFake{},
		CallbacksEnabled: true,
		Security:         appIncidentSecurityFake{},
		Users:            appIncidentUsersFake{},
		Audit:            appIncidentAuditFake{},
	}
	tests := []struct {
		name   string
		mutate func(*incidentRuntimeOptions)
	}{
		{name: "security", mutate: func(o *incidentRuntimeOptions) { o.Security = nil }},
		{name: "users", mutate: func(o *incidentRuntimeOptions) { o.Users = nil }},
		{name: "audit", mutate: func(o *incidentRuntimeOptions) { o.Audit = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := base
			tc.mutate(&opts)
			if runtime, err := openIncidentRuntime(context.Background(), opts); err == nil {
				if runtime != nil {
					_ = runtime.Close()
				}
				t.Fatalf("callbacks opened with missing %s authority", tc.name)
			}
		})
	}
}

func TestIncidentRuntimeServePublishesUnexpectedExitAndFencesOperations(t *testing.T) {
	workflow := &appIncidentWorkflowFake{}
	runtime := &IncidentRuntime{Workflow: workflow, done: make(chan struct{})}
	var callbackErr error
	runtime.serve(context.Background(), func(err error) { callbackErr = err })
	if callbackErr == nil || !strings.Contains(callbackErr.Error(), "stopped unexpectedly") {
		t.Fatalf("callback error=%v", callbackErr)
	}
	select {
	case <-runtime.done:
	default:
		t.Fatal("serve completion was not published")
	}
	if err := runtime.operational(); !errors.Is(err, ErrIncidentRuntimeUnavailable) || !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Fatalf("operational error=%v", err)
	}
}

func TestIncidentRuntimeServeErrorCallbackAndNilCallbackBranches(t *testing.T) {
	boom := errors.New("serve boom")
	withCallback := &IncidentRuntime{Workflow: &appIncidentWorkflowFake{serveErr: boom}, done: make(chan struct{})}
	calls := 0
	withCallback.serve(context.Background(), func(err error) {
		calls++
		if !errors.Is(err, boom) {
			t.Fatalf("callback error=%v want=%v", err, boom)
		}
	})
	if calls != 1 {
		t.Fatalf("serve-error callback calls=%d want=1", calls)
	}

	withoutCallback := &IncidentRuntime{Workflow: &appIncidentWorkflowFake{serveErr: boom}, done: make(chan struct{})}
	withoutCallback.serve(context.Background(), nil)
	if err := withoutCallback.operational(); !errors.Is(err, boom) {
		t.Fatalf("serve error was not preserved without callback: %v", err)
	}
}

func TestIncidentRuntimeLifecycleCancellationIsNotServeFailure(t *testing.T) {
	boom := errors.New("provider returned while shutting down")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := &IncidentRuntime{Workflow: &appIncidentWorkflowFake{serveErr: boom}, done: make(chan struct{})}
	calls := 0
	runtime.serve(ctx, func(error) { calls++ })
	if calls != 0 {
		t.Fatalf("normal lifecycle cancellation invoked error callback %d times", calls)
	}
	runtime.mu.RLock()
	serveErr := runtime.serveErr
	runtime.mu.RUnlock()
	if serveErr != nil {
		t.Fatalf("normal lifecycle cancellation recorded serve error: %v", serveErr)
	}
}

func TestIncidentRuntimeStartIsDeterministicAndCloseIsClean(t *testing.T) {
	runtime, err := openIncidentRuntime(context.Background(), incidentRuntimeOptions{
		Config:   memoryIncidentConfig(),
		Notifier: &appIncidentNotifierFake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	trigger := validAppIncidentTrigger()
	first, err := runtime.Start(context.Background(), trigger)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.Start(context.Background(), trigger)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.ID != second.ID || first.ID != domainincident.ExecutionID(trigger) {
		t.Fatalf("execution ids first=%q second=%q expected=%q", first.ID, second.ID, domainincident.ExecutionID(trigger))
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("normal cancellation surfaced as failure: %v", err)
	}
	if _, err := runtime.Start(context.Background(), trigger); err == nil {
		t.Fatal("closed runtime accepted new trigger")
	}
}

func TestIncidentRuntimeOperationalPreservesServeFailure(t *testing.T) {
	boom := errors.New("serve failed")
	runtime := &IncidentRuntime{Workflow: &appIncidentWorkflowFake{}, stopped: true, serveErr: boom}
	err := runtime.operational()
	if !errors.Is(err, ErrIncidentRuntimeUnavailable) || !errors.Is(err, boom) {
		t.Fatalf("operational error=%v must preserve unavailable and serve failure", err)
	}
}

func TestIncidentRuntimeDelegatesOnlyWhileOperational(t *testing.T) {
	workflow := &appIncidentWorkflowFake{
		startResult:   &adgo.Execution{ID: "start-ok"},
		ownerResult:   &adgo.Execution{ID: "owner-ok"},
		resolveResult: &adgo.Execution{ID: "resolve-ok"},
	}
	runtime := &IncidentRuntime{Workflow: workflow}
	if got, err := runtime.Start(context.Background(), validAppIncidentTrigger()); err != nil || got.ID != "start-ok" {
		t.Fatalf("start got=%v err=%v", got, err)
	}
	if got, err := runtime.OwnerResponse(context.Background(), "exec", "evt", map[string]any{"ok": true}); err != nil || got.ID != "owner-ok" {
		t.Fatalf("owner response got=%v err=%v", got, err)
	}
	if got, err := runtime.ResolveOwnerCallbackDecision(context.Background(), "exec", "evt", domainincident.DecisionAcknowledge, "usr-1", "ok", nil); err != nil || got.ID != "resolve-ok" {
		t.Fatalf("resolve got=%v err=%v", got, err)
	}
	if workflow.startCalls != 1 || workflow.ownerCalls != 1 || workflow.resolveCalls != 1 {
		t.Fatalf("delegation calls start=%d owner=%d resolve=%d", workflow.startCalls, workflow.ownerCalls, workflow.resolveCalls)
	}

	runtime.closed = true
	if _, err := runtime.Start(context.Background(), validAppIncidentTrigger()); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("closed start error=%v", err)
	}
	if _, err := runtime.OwnerResponse(context.Background(), "exec", "evt", nil); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("closed owner error=%v", err)
	}
	if _, err := runtime.ResolveOwnerCallbackDecision(context.Background(), "exec", "evt", domainincident.DecisionAcknowledge, "usr-1", "", nil); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("closed resolve error=%v", err)
	}
	if workflow.startCalls != 1 || workflow.ownerCalls != 1 || workflow.resolveCalls != 1 {
		t.Fatalf("closed runtime delegated calls start=%d owner=%d resolve=%d", workflow.startCalls, workflow.ownerCalls, workflow.resolveCalls)
	}
}

func TestIncidentRuntimeClosePreservesServeErrorAndRunsOnce(t *testing.T) {
	serveBoom := errors.New("serve boom")
	closeBoom := errors.New("close boom")
	workflow := &appIncidentWorkflowFake{closeErr: closeBoom}
	done := make(chan struct{})
	close(done)
	cancelCalls := 0
	runtime := &IncidentRuntime{
		Workflow: workflow,
		cancel:   func() { cancelCalls++ },
		done:     done,
		serveErr: serveBoom,
		stopped:  true,
	}
	for i := 0; i < 2; i++ {
		err := runtime.Close()
		if !errors.Is(err, serveBoom) || errors.Is(err, closeBoom) {
			t.Fatalf("close %d error=%v; serve error must win", i, err)
		}
	}
	if cancelCalls != 1 || workflow.closeCalls != 1 {
		t.Fatalf("close lifecycle calls cancel=%d workflow=%d", cancelCalls, workflow.closeCalls)
	}
}

func TestIncidentRuntimeCloseReturnsWorkflowErrorWhenServeWasClean(t *testing.T) {
	closeBoom := errors.New("close boom")
	workflow := &appIncidentWorkflowFake{closeErr: closeBoom}
	done := make(chan struct{})
	close(done)
	runtime := &IncidentRuntime{Workflow: workflow, cancel: func() {}, done: done}
	if err := runtime.Close(); !errors.Is(err, closeBoom) {
		t.Fatalf("close error=%v want=%v", err, closeBoom)
	}
}

func TestIncidentRuntimeNilCloseIsSafe(t *testing.T) {
	var runtime *IncidentRuntime
	if err := runtime.Close(); err != nil {
		t.Fatalf("nil close=%v", err)
	}
}

func TestIncidentRuntimeRootIsStableSiblingOfDatabase(t *testing.T) {
	db := filepath.Join("var", "lib", "sentinel", "sentinel.db")
	want := filepath.Join("var", "lib", "sentinel", "orchestration", "incident")
	if got := incidentRuntimeRoot("  " + db + "  "); got != want {
		t.Fatalf("root=%q want=%q", got, want)
	}
}

func validIncidentDependencyApp() *App {
	cfg := config.Default()
	cfg.Telegram.Enabled = true
	db := &sql.DB{}
	return &App{
		Config:   cfg,
		DB:       db,
		Users:    auth.NewUserStore(db),
		Audit:    repository.NewAuditStore(db),
		Telegram: &tgsvc.Service{Client: &tgapi.Client{}},
		runCtx:   context.Background(),
		Health:   health.NewRegistry(),
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestIncidentRuntimeDependencyMatrixFailsClosedBeforeOpen(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		mutate func(*App)
	}{
		{name: "database", want: "database unavailable", mutate: func(a *App) { a.DB = nil }},
		{name: "users", want: "user store unavailable", mutate: func(a *App) { a.Users = nil }},
		{name: "audit", want: "audit store unavailable", mutate: func(a *App) { a.Audit = nil }},
		{name: "telegram service", want: "Telegram service unavailable", mutate: func(a *App) { a.Telegram = nil }},
		{name: "telegram client", want: "Telegram client unavailable", mutate: func(a *App) { a.Telegram.Client = nil }},
		{name: "lifecycle", want: "lifecycle context unavailable", mutate: func(a *App) { a.runCtx = nil }},
		{name: "health", want: "health registry unavailable", mutate: func(a *App) { a.Health = nil }},
		{name: "logger", want: "logger unavailable", mutate: func(a *App) { a.Log = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := validIncidentDependencyApp()
			tc.mutate(a)
			called := false
			err := a.startIncidentRuntimeWith(func(context.Context, incidentRuntimeOptions) (*IncidentRuntime, error) {
				called = true
				return nil, nil
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want substring %q", err, tc.want)
			}
			if called {
				t.Fatal("runtime opener called despite missing dependency")
			}
		})
	}
}

func TestStartIncidentRuntimeCompositionAndOpenFailures(t *testing.T) {
	t.Run("disabled skips dependencies and opener", func(t *testing.T) {
		a := &App{Config: config.Default()}
		called := false
		if err := a.startIncidentRuntimeWith(func(context.Context, incidentRuntimeOptions) (*IncidentRuntime, error) {
			called = true
			return nil, errors.New("must not run")
		}); err != nil {
			t.Fatal(err)
		}
		if called || a.IncidentRuntime != nil {
			t.Fatal("disabled notifier created incident runtime")
		}
	})

	t.Run("nil app", func(t *testing.T) {
		var a *App
		if err := a.startIncidentRuntimeWith(func(context.Context, incidentRuntimeOptions) (*IncidentRuntime, error) { return nil, nil }); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("nil opener", func(t *testing.T) {
		a := validIncidentDependencyApp()
		if err := a.startIncidentRuntimeWith(nil); err == nil || !strings.Contains(err.Error(), "opener unavailable") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("opener error", func(t *testing.T) {
		a := validIncidentDependencyApp()
		boom := errors.New("open boom")
		err := a.startIncidentRuntimeWith(func(context.Context, incidentRuntimeOptions) (*IncidentRuntime, error) { return nil, boom })
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "open incident orchestration") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("nil runtime", func(t *testing.T) {
		a := validIncidentDependencyApp()
		err := a.startIncidentRuntimeWith(func(context.Context, incidentRuntimeOptions) (*IncidentRuntime, error) { return nil, nil })
		if err == nil || !strings.Contains(err.Error(), "returned nil runtime") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("wires notifier callback authority and health", func(t *testing.T) {
		a := validIncidentDependencyApp()
		a.Config.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
		a.Config.Security.Callbacks.Enabled = true
		a.CallbackSecurity = appIncidentSecurityFake{}
		workflow := &appIncidentWorkflowFake{}
		runtime := &IncidentRuntime{Workflow: workflow}
		called := false
		err := a.startIncidentRuntimeWith(func(parent context.Context, opts incidentRuntimeOptions) (*IncidentRuntime, error) {
			called = true
			if parent != a.runCtx {
				t.Fatal("runtime did not receive app lifecycle context")
			}
			if !opts.CallbacksEnabled || opts.Security == nil || opts.Users != a.Users || opts.Audit != a.Audit {
				t.Fatalf("callback wiring opts=%+v", opts)
			}
			notifier, ok := opts.Notifier.(*tgsvc.DurableNotifier)
			if !ok || notifier.Sender == nil || notifier.Pairings == nil || notifier.Users != a.Users || notifier.Deliveries == nil {
				t.Fatalf("durable notifier not fully wired: %#v", opts.Notifier)
			}
			wantRoot := incidentRuntimeRoot(a.Config.Database.Path)
			if opts.Config.Production.Root != wantRoot {
				t.Fatalf("runtime root=%q want=%q", opts.Config.Production.Root, wantRoot)
			}
			if opts.OnServeError == nil {
				t.Fatal("serve error observer missing")
			}
			return runtime, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !called || a.IncidentRuntime != runtime {
			t.Fatal("runtime was not installed")
		}
		component, ok := a.Health.Get("incident_orchestration")
		if !ok || component.Status != health.Healthy {
			t.Fatalf("health=%+v present=%v", component, ok)
		}
	})
}

func TestAppIncidentRuntimeConfigurationGate(t *testing.T) {
	cfg := config.Default()
	a := &App{Config: cfg}
	if err := a.startIncidentRuntime(); err != nil {
		t.Fatalf("disabled telegram/callback runtime should stay disabled: %v", err)
	}
	if a.IncidentRuntime != nil {
		t.Fatal("disabled production notifier created incident runtime")
	}

	cfg.Security.Callbacks.Enabled = true
	a = &App{Config: cfg}
	if err := a.startIncidentRuntime(); err != nil {
		t.Fatalf("callback security without a notifier should remain transport-inert: %v", err)
	}
	if a.IncidentRuntime != nil {
		t.Fatal("callback security alone created a production incident runtime")
	}
}

func TestAppStartIncidentFailsClosedForNilAppAndMissingRuntime(t *testing.T) {
	var nilApp *App
	if _, err := nilApp.StartIncident(context.Background(), validAppIncidentTrigger()); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("nil app error=%v", err)
	}
	a := &App{}
	if _, err := a.StartIncident(context.Background(), validAppIncidentTrigger()); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("missing runtime error=%v", err)
	}
}

func TestAppStartIncidentDelegatesToRuntime(t *testing.T) {
	workflow := &appIncidentWorkflowFake{startResult: &adgo.Execution{ID: "delegated"}}
	a := &App{IncidentRuntime: &IncidentRuntime{Workflow: workflow}}
	got, err := a.StartIncident(context.Background(), validAppIncidentTrigger())
	if err != nil || got.ID != "delegated" || workflow.startCalls != 1 {
		t.Fatalf("execution=%v error=%v calls=%d", got, err, workflow.startCalls)
	}
}
