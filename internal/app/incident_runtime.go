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

type incidentWorkflowRuntime interface {
	Start(context.Context, incident.Trigger) (*adgo.Execution, error)
	Serve(context.Context) error
	OwnerResponse(context.Context, string, string, any) (*adgo.Execution, error)
	ResolveOwnerCallbackDecision(context.Context, string, string, incident.Decision, string, string, any) (*adgo.Execution, error)
	Close() error
}

// IncidentRuntime owns one durable ADGO incident service and every authority
// that may resume it. In particular, CallbackIngress delegates back through
// this wrapper rather than holding an unrelated workflow instance.
type IncidentRuntime struct {
	Workflow  incidentWorkflowRuntime
	Callbacks *orchestrationincident.CallbackIngress

	cancel context.CancelFunc
	done   chan struct{}

	mu        sync.RWMutex
	serveErr  error
	stopped   bool
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

type incidentRuntimeOpener func(context.Context, incidentRuntimeOptions) (*IncidentRuntime, error)

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
		done:     make(chan struct{}),
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
	rawErr := r.Workflow.Serve(ctx)
	serveErr := classifyIncidentServeExit(ctx.Err(), rawErr)

	r.mu.Lock()
	r.stopped = true
	r.serveErr = serveErr
	r.mu.Unlock()

	if serveErr != nil && onServeError != nil {
		onServeError(serveErr)
	}
	close(r.done)
}

func classifyIncidentServeExit(lifecycleErr, serveErr error) error {
	if lifecycleErr != nil {
		return nil
	}
	if serveErr == nil {
		return errors.New("application: incident orchestration serve loop stopped unexpectedly")
	}
	return serveErr
}

func (r *IncidentRuntime) operational() error {
	if r == nil || r.Workflow == nil {
		return ErrIncidentRuntimeUnavailable
	}
	r.mu.RLock()
	err := r.serveErr
	stopped := r.stopped
	closed := r.closed
	r.mu.RUnlock()
	if closed || stopped {
		if err != nil {
			return errors.Join(ErrIncidentRuntimeUnavailable, fmt.Errorf("incident serve loop stopped: %w", err))
		}
		return ErrIncidentRuntimeUnavailable
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

// Close is defined for runtimes returned by openIncidentRuntime. Construction
// establishes non-nil Workflow/cancel/done fields, so shutdown intentionally
// relies on those invariants rather than silently accepting partial runtimes.
func (r *IncidentRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()

		r.cancel()
		<-r.done

		r.mu.RLock()
		serveErr := r.serveErr
		r.mu.RUnlock()
		if serveErr != nil {
			r.closeErr = serveErr
		}
		if err := r.Workflow.Close(); err != nil && r.closeErr == nil {
			r.closeErr = err
		}
	})
	return r.closeErr
}

func validateIncidentRuntimeDependencies(a *App) error {
	switch {
	case a.DB == nil:
		return errors.New("application: incident runtime database unavailable")
	case a.Users == nil:
		return errors.New("application: incident runtime user store unavailable")
	case a.Audit == nil:
		return errors.New("application: incident runtime audit store unavailable")
	case a.Telegram == nil:
		return errors.New("application: incident runtime Telegram service unavailable")
	case a.Telegram.Client == nil:
		return errors.New("application: incident runtime Telegram client unavailable")
	case a.runCtx == nil:
		return errors.New("application: incident runtime lifecycle context unavailable")
	case a.Health == nil:
		return errors.New("application: incident runtime health registry unavailable")
	case a.Log == nil:
		return errors.New("application: incident runtime logger unavailable")
	default:
		return nil
	}
}

// startIncidentRuntime wires the currently supported production notifier.
// CallbackSecurity may exist independently, but no CallbackIngress is exposed
// unless this production workflow and its durable notifier are both available.
func (a *App) startIncidentRuntime() error {
	return a.startIncidentRuntimeWith(openIncidentRuntime)
}

func (a *App) startIncidentRuntimeWith(openRuntime incidentRuntimeOpener) error {
	if a == nil {
		return ErrIncidentRuntimeUnavailable
	}
	if !a.Config.Telegram.Enabled {
		return nil
	}
	if err := validateIncidentRuntimeDependencies(a); err != nil {
		return err
	}
	if openRuntime == nil {
		return errors.New("application: incident runtime opener unavailable")
	}

	notifier := &tgsvc.DurableNotifier{
		Sender:     a.Telegram.Client,
		Pairings:   a.Telegram.Pairings,
		Users:      a.Users,
		Deliveries: tgsvc.NotificationDeliveryStore{DB: a.DB},
	}
	root := incidentRuntimeRoot(a.Config.Database.Path)
	config := orchestrationincident.DefaultConfig(root)
	runtime, err := openRuntime(a.runCtx, incidentRuntimeOptions{
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
	if runtime == nil {
		return errors.New("application: incident runtime opener returned nil runtime")
	}
	a.IncidentRuntime = runtime
	a.Health.Set("incident_orchestration", "HEALTHY", "", "")
	return nil
}

// StartIncident is the explicit application boundary for reviewed domain
// triggers. Raw event-to-trigger policy is intentionally not inferred here.
func (a *App) StartIncident(ctx context.Context, trigger incident.Trigger) (*adgo.Execution, error) {
	if a == nil {
		return nil, ErrIncidentRuntimeUnavailable
	}
	if a.IncidentRuntime == nil {
		return nil, ErrIncidentRuntimeUnavailable
	}
	return a.IncidentRuntime.Start(ctx, trigger)
}

func incidentRuntimeRoot(databasePath string) string {
	return filepath.Join(filepath.Dir(strings.TrimSpace(databasePath)), "orchestration", "incident")
}
