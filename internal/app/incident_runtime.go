package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	orchestrationincident "github.com/Homiakus/Home_Sentinel/internal/orchestration/incident"
	tgsvc "github.com/Homiakus/Home_Sentinel/internal/telegram"
	"github.com/Homiakus/axiom/adgo"
)

var ErrIncidentRuntimeUnavailable = errors.New("application: incident orchestration runtime unavailable")

// IncidentRuntime owns one durable ADGO incident service and every authority
// that may resume it. In particular, CallbackIngress delegates back through
// this wrapper rather than holding an unrelated workflow instance.
type IncidentRuntime struct {
	Workflow  *orchestrationincident.Service
	Callbacks *orchestrationincident.CallbackIngress

	cancel context.CancelFunc
	done   chan error

	mu        sync.RWMutex
	serveErr  error
	closed    bool
	closeOnce sync.Once
	closeErr  error
}

type incidentRuntimeOptions struct {
	Config           orchestrationincident.Config
	Notifier         gateway.Notifier
	CallbacksEnabled bool
	Security         CallbackSecurity
	Users            orchestrationincident.CallbackUserStore
	Audit            orchestrationincident.CallbackAuditStore
	OnServeError     func(error)
}

func openIncidentRuntime(parent context.Context, opts incidentRuntimeOptions) (*IncidentRuntime, error) {
	if parent == nil {
		return nil, errors.New("application: incident runtime parent context is required")
	}
	if opts.Notifier == nil {
		return nil, errors.New("application: durable incident notifier is required")
	}
	if opts.CallbacksEnabled && (opts.Security == nil || opts.Users == nil || opts.Audit == nil) {
		return nil, errors.New("application: enabled incident callbacks require security, users and audit")
	}

	workflow, err := orchestrationincident.Open(opts.Config, orchestrationincident.Dependencies{Notifier: opts.Notifier})
	if err != nil {
		return nil, err
	}
	serveCtx, cancel := context.WithCancel(parent)
	runtime := &IncidentRuntime{
		Workflow: workflow,
		cancel:   cancel,
		done:     make(chan error, 1),
	}
	if opts.CallbacksEnabled {
		runtime.Callbacks = &orchestrationincident.CallbackIngress{
			Security: opts.Security,
			Users:    opts.Users,
			Audit:    opts.Audit,
			Workflow: runtime,
		}
	}
	go runtime.serve(serveCtx, opts.OnServeError)
	return runtime, nil
}

func (r *IncidentRuntime) serve(ctx context.Context, onServeError func(error)) {
	err := r.Workflow.Serve(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		r.mu.Lock()
		r.serveErr = err
		r.mu.Unlock()
		if onServeError != nil {
			onServeError(err)
		}
	}
	r.done <- err
	close(r.done)
}

func (r *IncidentRuntime) operational() error {
	if r == nil || r.Workflow == nil {
		return ErrIncidentRuntimeUnavailable
	}
	r.mu.RLock()
	err := r.serveErr
	closed := r.closed
	r.mu.RUnlock()
	if closed {
		return ErrIncidentRuntimeUnavailable
	}
	if err != nil {
		return errors.Join(ErrIncidentRuntimeUnavailable, fmt.Errorf("incident serve loop stopped: %w", err))
	}
	return nil
}

func (r *IncidentRuntime) Start(ctx context.Context, trigger incident.Trigger) (*adgo.Execution, error) {
	if err := r.operational(); err != nil {
		return nil, err
	}
	return r.Workflow.Start(ctx, trigger)
}

func (r *IncidentRuntime) OwnerResponse(ctx context.Context, executionID, eventID string, payload any) (*adgo.Execution, error) {
	if err := r.operational(); err != nil {
		return nil, err
	}
	return r.Workflow.OwnerResponse(ctx, executionID, eventID, payload)
}

func (r *IncidentRuntime) ResolveOwnerCallbackDecision(
	ctx context.Context,
	executionID string,
	eventID string,
	decision incident.Decision,
	actor string,
	reason string,
	payload any,
) (*adgo.Execution, error) {
	if err := r.operational(); err != nil {
		return nil, err
	}
	return r.Workflow.ResolveOwnerCallbackDecision(ctx, executionID, eventID, decision, actor, reason, payload)
}

func (r *IncidentRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()
		if r.cancel != nil {
			r.cancel()
		}
		if r.done != nil {
			if err, ok := <-r.done; ok && err != nil && !errors.Is(err, context.Canceled) {
				r.closeErr = err
			}
		}
		if r.Workflow != nil {
			if err := r.Workflow.Close(); err != nil && r.closeErr == nil {
				r.closeErr = err
			}
		}
	})
	return r.closeErr
}

// startIncidentRuntime wires the currently supported production notifier.
// Telegram disabled means no production orchestration today; callback authority
// cannot be enabled in that state because medium/high workflows could not have
// produced the owner notification that precedes their callback nodes.
func (a *App) startIncidentRuntime() error {
	if a == nil {
		return ErrIncidentRuntimeUnavailable
	}
	if !a.Config.Telegram.Enabled {
		// CallbackSecurity may exist independently as a cryptographic authority.
		// Without a production notifier we intentionally expose no incident
		// runtime and therefore no CallbackIngress.
		return nil
	}
	if a.DB == nil || a.Users == nil || a.Audit == nil || a.Telegram == nil || a.Telegram.Client == nil {
		return errors.New("application: incident runtime dependencies unavailable")
	}

	notifier := &tgsvc.DurableNotifier{
		Sender:     a.Telegram.Client,
		Pairings:   tgsvc.PairingStore{DB: a.DB},
		Users:      a.Users,
		Deliveries: tgsvc.NotificationDeliveryStore{DB: a.DB},
	}
	root := incidentRuntimeRoot(a.Config.Database.Path)
	config := orchestrationincident.DefaultConfig(root)
	runtime, err := openIncidentRuntime(a.runCtx, incidentRuntimeOptions{
		Config:           config,
		Notifier:         notifier,
		CallbacksEnabled: a.Config.Security.Callbacks.Enabled,
		Security:         a.CallbackSecurity,
		Users:            a.Users,
		Audit:            a.Audit,
		OnServeError: func(serveErr error) {
			a.Health.Set("incident_orchestration", "DEGRADED", "INCIDENT_RUNTIME_STOPPED", "incident orchestration runtime stopped unexpectedly")
			a.Log.Error("incident orchestration runtime stopped", "error", serveErr)
		},
	})
	if err != nil {
		return fmt.Errorf("open incident orchestration: %w", err)
	}
	a.IncidentRuntime = runtime
	a.Health.Set("incident_orchestration", "HEALTHY", "", "")
	return nil
}

// StartIncident is the explicit application boundary for reviewed domain
// triggers. Raw event-to-trigger policy is intentionally not inferred here.
func (a *App) StartIncident(ctx context.Context, trigger incident.Trigger) (*adgo.Execution, error) {
	if a == nil || a.IncidentRuntime == nil {
		return nil, ErrIncidentRuntimeUnavailable
	}
	return a.IncidentRuntime.Start(ctx, trigger)
}

func incidentRuntimeRoot(databasePath string) string {
	return filepath.Join(filepath.Dir(strings.TrimSpace(databasePath)), "orchestration", "incident")
}
