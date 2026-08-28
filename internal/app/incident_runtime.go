package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/health"
	orincident "github.com/Homiakus/Home_Sentinel/internal/orchestration/incident"
	tgsvc "github.com/Homiakus/Home_Sentinel/internal/telegram"
)

const incidentRuntimeShutdownTimeout = 10 * time.Second

func (a *App) startIncidentRuntime() error {
	if a == nil {
		return errors.New("incident runtime: application is required")
	}
	if a.IncidentRuntime != nil {
		return errors.New("incident runtime: already started")
	}
	if a.Telegram == nil {
		// Callback signing/verification authority is intentionally independent
		// from notification transport. Without Telegram there is no production
		// incident workflow, so the narrower callback ingress remains absent.
		return nil
	}
	if err := a.validateIncidentRuntimeDependencies(); err != nil {
		return err
	}

	notifier := &tgsvc.DurableNotifier{
		Sender:     a.Telegram.Client,
		Pairings:   a.Telegram.Pairings,
		Users:      a.Users,
		Deliveries: tgsvc.NotificationDeliveryStore{DB: a.DB},
	}
	root := filepath.Join(filepath.Dir(a.Config.Database.Path), "orchestration", "incident")
	runtime, err := orincident.Open(orincident.DefaultConfig(root), orincident.Dependencies{Notifier: notifier})
	if err != nil {
		return fmt.Errorf("incident runtime: open: %w", err)
	}
	a.IncidentRuntime = runtime
	if a.CallbackSecurity != nil {
		a.IncidentCallbacks = &orincident.CallbackIngress{
			Security: a.CallbackSecurity,
			Users:    a.Users,
			Audit:    a.Audit,
			Workflow: runtime,
		}
	}
	a.incidentServeDone = make(chan error, 1)
	a.Health.Set("incident_runtime", health.Starting, "", "")
	go a.serveIncidentRuntime(runtime, a.incidentServeDone)
	return nil
}

func (a *App) validateIncidentRuntimeDependencies() error {
	if a.Telegram.Client == nil {
		return errors.New("incident runtime: Telegram client unavailable")
	}
	if a.DB == nil {
		return errors.New("incident runtime: database unavailable")
	}
	if a.Users == nil {
		return errors.New("incident runtime: user store unavailable")
	}
	if a.Audit == nil {
		return errors.New("incident runtime: audit store unavailable")
	}
	if a.runCtx == nil {
		return errors.New("incident runtime: lifecycle context unavailable")
	}
	if a.Health == nil {
		return errors.New("incident runtime: health registry unavailable")
	}
	return nil
}

func (a *App) serveIncidentRuntime(runtime *orincident.Service, done chan<- error) {
	err := runtime.Serve(a.runCtx)
	err, unexpected := classifyIncidentServeExit(a.runCtx.Err(), err)
	if unexpected {
		a.Health.Set("incident_runtime", health.Degraded, "INCIDENT_RUNTIME_STOPPED", "durable incident runtime stopped unexpectedly")
		if a.Log != nil {
			a.Log.Error("durable incident runtime stopped", "error", err)
		}
	}
	done <- err
	close(done)
}

func classifyIncidentServeExit(lifecycleErr, serveErr error) (error, bool) {
	if lifecycleErr != nil {
		return serveErr, false
	}
	if serveErr == nil {
		serveErr = errors.New("incident runtime: serve loop stopped unexpectedly")
	}
	return serveErr, true
}

func (a *App) stopIncidentRuntime() error {
	if a == nil {
		return nil
	}
	if a.IncidentRuntime == nil {
		return nil
	}
	var lifecycleErr error
	if a.runCtx != nil {
		lifecycleErr = a.runCtx.Err()
	}
	if err := waitIncidentServe(a.incidentServeDone, lifecycleErr, incidentRuntimeShutdownTimeout); err != nil {
		return err
	}
	if err := a.IncidentRuntime.Close(); err != nil {
		return fmt.Errorf("incident runtime: close: %w", err)
	}
	a.IncidentRuntime = nil
	a.IncidentCallbacks = nil
	a.incidentServeDone = nil
	return nil
}

func waitIncidentServe(done <-chan error, lifecycleErr error, timeout time.Duration) error {
	if done == nil {
		return errors.New("incident runtime: serve completion channel missing")
	}
	if timeout <= 0 {
		return errors.New("incident runtime: shutdown timeout must be positive")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if lifecycleErr != nil {
			return nil
		}
		if err == nil {
			return errors.New("incident runtime: serve loop stopped unexpectedly")
		}
		return fmt.Errorf("incident runtime: serve stopped: %w", err)
	case <-timer.C:
		return errors.New("incident runtime: serve did not stop after cancellation")
	}
}
