package door

import (
	"context"
	"testing"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func openMemory(t *testing.T, controller gateway.DoorController) *Service {
	t.Helper()
	service, err := Open(Config{
		Production: adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:   "door-test", WorkerConcurrency: 1,
	}, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestPlanCompiles(t *testing.T) {
	if _, err := CompilePlan(); err != nil {
		t.Fatalf("compile door plan: %v", err)
	}
}

func TestLockUsesDesiredStateAndStartOrLoad(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockUnlocked})
	service := openMemory(t, controller)
	request := domainaction.DoorRequest{RequestID: "lock-1", DoorID: "front", Desired: gateway.LockLocked, RequestedBy: "policy"}

	first, err := service.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	duplicate, err := service.Start(ctx, request)
	if err != nil {
		t.Fatalf("duplicate start: %v", err)
	}
	if first.ID != duplicate.ID {
		t.Fatal("door StartOrLoad is not idempotent")
	}
	completed, err := service.Drive(ctx, first.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || controller.Applied != 1 {
		t.Fatalf("lock action failed: status=%s applied=%d", completed.Status, controller.Applied)
	}
	state, _ := controller.LockState(ctx, "front")
	if state != gateway.LockLocked {
		t.Fatalf("door not locked: %s", state)
	}
}

func TestUnlockRequiresHumanApproval(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	service := openMemory(t, controller)
	request := domainaction.DoorRequest{RequestID: "unlock-1", DoorID: "front", Desired: gateway.LockUnlocked, RequestedBy: "owner"}
	execution, err := service.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeApproveUnlock] != UnlockApprovalEvent {
		t.Fatalf("unlock bypassed human gate: status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}
	completed, err := service.ResolveUnlockApproval(ctx, execution.ID, domainaction.ApprovalApprove, "owner", "open for visitor")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("unlock did not complete: %s", completed.Status)
	}
	state, _ := controller.LockState(ctx, "front")
	if state != gateway.LockUnlocked {
		t.Fatalf("door not unlocked: %s", state)
	}
}

func TestAmbiguousButAppliedIsVerifiedAutomatically(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockUnlocked})
	controller.SetNextAmbiguity(gatewayfake.DoorAmbiguousApplied)
	service := openMemory(t, controller)
	request := domainaction.DoorRequest{RequestID: "ambiguous-applied", DoorID: "front", Desired: gateway.LockLocked}
	execution, _ := service.Start(ctx, request)
	completed, err := service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || controller.Applied != 1 {
		t.Fatalf("verified ambiguous write not accepted: status=%s applied=%d", completed.Status, controller.Applied)
	}
}

func TestAmbiguousUnknownEntersDurableReconciliation(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	controller.SetNextAmbiguity(gatewayfake.DoorAmbiguousNotApplied)
	service := openMemory(t, controller)
	request := domainaction.DoorRequest{RequestID: "ambiguous-unlock", DoorID: "front", Desired: gateway.LockUnlocked}
	execution, _ := service.Start(ctx, request)
	execution, err := service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to approval: %v", err)
	}
	execution, err = service.ResolveUnlockApproval(ctx, execution.ID, domainaction.ApprovalApprove, "owner", "approved")
	if err != nil {
		t.Fatalf("approve unlock: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeApplyUnlock] != "Reconcile:"+NodeApplyUnlock {
		t.Fatalf("ambiguous write did not enter reconciliation: status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}

	completed, err := service.ResolveReconciliation(
		ctx, execution.ID, NodeApplyUnlock, domainaction.ReconcileRetry, "operator", "device reports still locked",
	)
	if err != nil {
		t.Fatalf("reconcile retry: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || controller.Applied != 1 {
		t.Fatalf("reconciled retry failed: status=%s applied=%d", completed.Status, controller.Applied)
	}
}
