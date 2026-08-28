package siren

import (
	"context"
	"errors"
	"testing"
	"time"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func openMemory(t *testing.T, controller *gatewayfake.SirenController, duration time.Duration) *Service {
	t.Helper()
	service, err := Open(Config{
		Production: adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:   "siren-test", WorkerConcurrency: 1, MaxActivationDuration: duration,
	}, Dependencies{Siren: controller})
	if err != nil {
		t.Fatalf("open siren: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestPlanCompiles(t *testing.T) {
	if _, err := CompilePlan(time.Second); err != nil {
		t.Fatalf("compile siren plan: %v", err)
	}
}

func TestSirenAutomaticallyStopsAtSafetyDeadline(t *testing.T) {
	controller := gatewayfake.NewSirenController(map[string]bool{"main": false})
	service := openMemory(t, controller, 15*time.Millisecond)
	ctx := context.Background()
	execution, err := service.Start(ctx, domainaction.SirenRequest{RequestID: "alarm-1", SirenID: "main", RequestedBy: "incident"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusWaiting {
		t.Fatalf("expected waiting on safety timer, got %s", execution.Status)
	}
	time.Sleep(25 * time.Millisecond)
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive after timer: %v", err)
	}
	if execution.Status != adgo.StatusCompleted {
		t.Fatalf("expected completed, got %s", execution.Status)
	}
	enabled, err := controller.Enabled(ctx, "main")
	if err != nil {
		t.Fatalf("read siren: %v", err)
	}
	if enabled || controller.Applied != 2 {
		t.Fatalf("safety timer failed: enabled=%v applied=%d", enabled, controller.Applied)
	}
}

func TestManualStopUsesCompensation(t *testing.T) {
	controller := gatewayfake.NewSirenController(map[string]bool{"main": false})
	service := openMemory(t, controller, time.Hour)
	ctx := context.Background()

	execution, err := service.Start(ctx, domainaction.SirenRequest{RequestID: "alarm-stop", SirenID: "main"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to safety timer: %v", err)
	}
	if execution.Status != adgo.StatusWaiting {
		t.Fatalf("expected waiting before manual stop, got %s", execution.Status)
	}
	enabled, err := controller.Enabled(ctx, "main")
	if err != nil {
		t.Fatalf("read enabled siren: %v", err)
	}
	if !enabled {
		t.Fatal("siren was not enabled before manual stop")
	}

	if _, err := service.Stop(ctx, execution.ID, "manual override"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopped, err := service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive cancellation: %v", err)
	}
	if stopped.Status != adgo.StatusCanceled {
		t.Fatalf("expected canceled execution, got %s", stopped.Status)
	}
	enabled, err = controller.Enabled(ctx, "main")
	if err != nil {
		t.Fatalf("read compensated siren: %v", err)
	}
	if enabled || controller.Applied != 2 {
		t.Fatalf("compensation failed: enabled=%v applied=%d", enabled, controller.Applied)
	}
}

func TestServeReturnsCanceledContext(t *testing.T) {
	controller := gatewayfake.NewSirenController(map[string]bool{"main": false})
	service := openMemory(t, controller, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := service.Serve(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("serve canceled context: %v", err)
	}
}
