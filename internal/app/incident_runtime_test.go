package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	orchestrationincident "github.com/Homiakus/Home_Sentinel/internal/orchestration/incident"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
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

func TestOpenIncidentRuntimeRequiresDurableNotifier(t *testing.T) {
	_, err := openIncidentRuntime(context.Background(), incidentRuntimeOptions{Config: memoryIncidentConfig()})
	if err == nil {
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

func TestOpenIncidentRuntimeEnabledCallbacksFailClosedOnMissingAuthority(t *testing.T) {
	_, err := openIncidentRuntime(context.Background(), incidentRuntimeOptions{
		Config:           memoryIncidentConfig(),
		Notifier:         &appIncidentNotifierFake{},
		CallbacksEnabled: true,
	})
	if err == nil {
		t.Fatal("callbacks opened without security/users/audit")
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

func TestIncidentRuntimeRootIsStableSiblingOfDatabase(t *testing.T) {
	db := filepath.Join("var", "lib", "sentinel", "sentinel.db")
	want := filepath.Join("var", "lib", "sentinel", "orchestration", "incident")
	if got := incidentRuntimeRoot(db); got != want {
		t.Fatalf("root=%q want=%q", got, want)
	}
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
	if err := a.startIncidentRuntime(); err == nil {
		t.Fatal("callback authority enabled without production notifier")
	}
}

func TestAppStartIncidentUnavailableFailsClosed(t *testing.T) {
	a := &App{}
	if _, err := a.StartIncident(context.Background(), validAppIncidentTrigger()); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("error=%v want ErrIncidentRuntimeUnavailable", err)
	}
}
