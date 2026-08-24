package camera

import (
	"context"
	"testing"
)

func TestDefinitionCompiles(t *testing.T) {
	if _, err := Definition().Compile(); err != nil {
		t.Fatalf("camera definition does not compile: %v", err)
	}
}

func TestLifecycleAndDisabledRecoveryGuard(t *testing.T) {
	ctx := context.Background()
	service, err := NewService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	const id = "camera-front"
	if err := service.Connected(ctx, id, "rtsp://front"); err != nil {
		t.Fatalf("connected: %v", err)
	}
	state, err := service.State(ctx, id)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != StateOnline || !state.Enabled {
		t.Fatalf("unexpected online state: %+v", state)
	}

	if err := service.Disable(ctx, id, "maintenance"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := service.Recovered(ctx, id, "late-probe"); err != nil {
		t.Fatalf("late recovery dispatch: %v", err)
	}
	state, err = service.State(ctx, id)
	if err != nil {
		t.Fatalf("state after disable: %v", err)
	}
	if state.Status != StateDisabled || state.Enabled {
		t.Fatalf("late recovery escaped disabled state: %+v", state)
	}

	if err := service.Enable(ctx, id, "maintenance-complete"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	state, err = service.State(ctx, id)
	if err != nil {
		t.Fatalf("state after enable: %v", err)
	}
	if state.Status != StateConnecting || !state.Enabled {
		t.Fatalf("unexpected enabled state: %+v", state)
	}
}
