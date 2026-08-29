package siren

import (
	"context"
	"testing"
	"time"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
)

func TestStopReturnsPersistedExecution(t *testing.T) {
	ctx := context.Background()
	service := openMemory(t, gatewayfake.NewSirenController(map[string]bool{"main": false}), time.Hour)
	execution, err := service.Start(ctx, domainaction.SirenRequest{RequestID: "stop-return-contract", SirenID: "main"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := service.Drive(ctx, execution.ID); err != nil {
		t.Fatalf("drive to safety wait: %v", err)
	}
	stopped, err := service.Stop(ctx, execution.ID, "operator stop")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if stopped == nil || stopped.ID != execution.ID {
		t.Fatalf("stop returned invalid execution: %+v", stopped)
	}
}
