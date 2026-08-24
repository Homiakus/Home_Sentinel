package door

import (
	"context"
	"errors"
	"testing"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
)

func TestSameDoorIsReservedAcrossIndependentExecutions(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{
		"front": gateway.LockLocked,
		"rear":  gateway.LockLocked,
	})
	service := openMemory(t, controller)

	firstRequest := domainaction.DoorRequest{
		RequestID: "unlock-owner-1", DoorID: "front", Desired: gateway.LockUnlocked,
	}
	first, err := service.Start(ctx, firstRequest)
	if err != nil {
		t.Fatalf("start first: %v", err)
	}
	first, err = service.Drive(ctx, first.ID)
	if err != nil {
		t.Fatalf("drive first: %v", err)
	}
	if first.WaitingFor[NodeApproveUnlock] != UnlockApprovalEvent {
		t.Fatalf("first execution is not holding human wait: %v", first.WaitingFor)
	}

	_, err = service.Start(ctx, domainaction.DoorRequest{
		RequestID: "lock-owner-2", DoorID: "front", Desired: gateway.LockLocked,
	})
	if !errors.Is(err, resourceguard.ErrBusy) {
		t.Fatalf("same door was not reserved: %v", err)
	}

	if _, err := service.Start(ctx, domainaction.DoorRequest{
		RequestID: "unlock-rear", DoorID: "rear", Desired: gateway.LockUnlocked,
	}); err != nil {
		t.Fatalf("different door should be independently reservable: %v", err)
	}

	completed, err := service.ResolveUnlockApproval(ctx, first.ID, domainaction.ApprovalReject, "owner", "deny")
	if err != nil {
		t.Fatalf("resolve first: %v", err)
	}
	if completed.Status == "" {
		t.Fatal("first execution has empty terminal status")
	}
	if _, err := service.Start(ctx, domainaction.DoorRequest{
		RequestID: "lock-after-release", DoorID: "front", Desired: gateway.LockLocked,
	}); err != nil {
		t.Fatalf("terminal execution did not release door reservation: %v", err)
	}
}
