package siren

import (
	"context"
	"errors"
	"testing"
	"time"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
)

func TestSameSirenCannotHaveTwoNonTerminalExecutions(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewSirenController(map[string]bool{"main": false, "garage": false})
	service := openMemory(t, controller, time.Hour)
	if _, err := service.Start(ctx, domainaction.SirenRequest{RequestID: "alarm-1", SirenID: "main"}); err != nil {
		t.Fatalf("start first: %v", err)
	}
	if _, err := service.Start(ctx, domainaction.SirenRequest{RequestID: "alarm-2", SirenID: "main"}); !errors.Is(err, resourceguard.ErrBusy) {
		t.Fatalf("same siren was not reserved: %v", err)
	}
	if _, err := service.Start(ctx, domainaction.SirenRequest{RequestID: "alarm-3", SirenID: "garage"}); err != nil {
		t.Fatalf("different siren blocked: %v", err)
	}
}
