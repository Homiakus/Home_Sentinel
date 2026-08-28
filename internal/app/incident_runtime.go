package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

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
	if a.Config.Security.Callbacks.Enabled && a.Telegram == nil {
		return errors.New("incident runtime: callbacks require enabled Telegram notification runtime")
	}
	if a.Telegram == nil {
		return nil
	}
	if a.Telegram.Client == nil || a.DB == nil || a.Users == nil || a.Audit == nil {
		return errors.New("incident runtime: production dependencies unavailable")
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
	a.Health.Set("incident_runtime", "STARTING", "", "")
	go a.serveIncidentRuntime(runtime, a.incidentServeDone)
	return nil
}

func (a *App) serveIncidentRuntime(runtime *orincident.Service, done chan<- error) {
	err := runtime.Serve(a.runCtx)
	if a.runCtx != nil && a.runCtx.Err() == nil {
		if err == nil {
			err = errors.New("incident runtime: serve loop stopped unexpectedly")
		}
		a.Health.Set("incident_runtime", "DEGRADED", "INCIDENT_RUNTIME_STOPPED", "durable incident runtime stopped unexpectedly")
		if a.Log != nil {
			a.Log.Error("durable incident runtime stopped", "error", err)
		}
	}
	done <- err
	close(done)
}

func (a *App) stopIncidentRuntime() error {
	if a == nil || a.IncidentRuntime == nil {
		return nil
	}
	if a.incidentServeDone != nil {
		timer := time.NewTimer(incidentRuntimeShutdownTimeout)
		defer timer.Stop()
		select {
		case err := <-a.incidentServeDone:
			if err != nil && !errors.Is(err, context.Canceled) && a.runCtx != nil && a.runCtx.Err() == nil {
				return fmt.Errorf("incident runtime: serve stopped: %w", err)
			}
		case <-timer.C:
			return errors.New("incident runtime: serve did not stop after cancellation")
		}
	}
	if err := a.IncidentRuntime.Close(); err != nil {
		return fmt.Errorf("incident runtime: close: %w", err)
	}
	a.IncidentRuntime = nil
	a.IncidentCallbacks = nil
	a.incidentServeDone = nil
	return nil
}
