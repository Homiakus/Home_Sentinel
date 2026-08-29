package door

import (
	"context"
	"errors"
	"testing"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestDoorV1PlanIdentityIsGolden(t *testing.T) {
	plan, err := CompilePlan()
	if err != nil {
		t.Fatalf("CompilePlan: %v", err)
	}
	if plan.ID != PlanID || plan.Version != PlanVersion || plan.Digest != doorV1PlanDigest {
		t.Fatalf(
			"door v1 identity drifted: got=%s/%s/%s want=%s/%s/%s",
			plan.ID, plan.Version, plan.Digest,
			PlanID, PlanVersion, doorV1PlanDigest,
		)
	}
}

func TestDoorPebbleReopenPreservesUnlockApprovalBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	cfg := DefaultConfig(root)

	first, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	request := domainaction.DoorRequest{RequestID: "upgrade-unlock", DoorID: "front", Desired: gateway.LockUnlocked, RequestedBy: "owner"}
	execution, err := first.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to approval: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeApproveUnlock] != UnlockApprovalEvent {
		t.Fatalf("approval wait status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}
	persistedDigest := execution.PlanDigest
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	second, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	completed, err := second.ResolveUnlockApproval(ctx, execution.ID, domainaction.ApprovalApprove, "owner", "approved after restart")
	if err != nil {
		t.Fatalf("resolve approval after restart: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || completed.PlanDigest != persistedDigest {
		t.Fatalf("completed status=%s digest=%s want digest=%s", completed.Status, completed.PlanDigest, persistedDigest)
	}
	state, err := controller.LockState(ctx, "front")
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != gateway.LockUnlocked || controller.Applied != 1 || controller.Calls != 1 {
		t.Fatalf("unlock replay mismatch: state=%s applied=%d calls=%d", state, controller.Applied, controller.Calls)
	}
}

func TestDoorPebbleReopenPreservesAmbiguousReconciliationBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	controller.SetNextAmbiguity(gatewayfake.DoorAmbiguousNotApplied)
	cfg := DefaultConfig(root)

	first, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	request := domainaction.DoorRequest{RequestID: "upgrade-reconcile", DoorID: "front", Desired: gateway.LockUnlocked}
	execution, err := first.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to approval: %v", err)
	}
	execution, err = first.ResolveUnlockApproval(ctx, execution.ID, domainaction.ApprovalApprove, "owner", "approved")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeApplyUnlock] != "Reconcile:"+NodeApplyUnlock {
		t.Fatalf("reconciliation wait status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}
	if controller.Applied != 0 || controller.Calls != 1 {
		t.Fatalf("unexpected physical result before restart: applied=%d calls=%d", controller.Applied, controller.Calls)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	second, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	completed, err := second.ResolveReconciliation(
		ctx, execution.ID, NodeApplyUnlock, domainaction.ReconcileRetry, "operator", "still locked after restart",
	)
	if err != nil {
		t.Fatalf("reconcile retry after restart: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("reconciled status=%s", completed.Status)
	}
	state, err := controller.LockState(ctx, "front")
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != gateway.LockUnlocked || controller.Applied != 1 || controller.Calls != 2 {
		t.Fatalf("reconciliation result state=%s applied=%d calls=%d", state, controller.Applied, controller.Calls)
	}
}

func TestDoorOpenRejectsUnknownNonTerminalBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	execution, err := first.Start(ctx, domainaction.DoorRequest{RequestID: "unknown-bundle", DoorID: "front", Desired: gateway.LockUnlocked})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanDigest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		return nil
	})
	if err != nil {
		t.Fatalf("inject unknown digest: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	_, err = Open(cfg, Dependencies{Door: controller})
	if !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("reopen error=%v want ErrUnknownExecutionBundle", err)
	}
}

func TestDoorOpenRejectsMismatchedNonTerminalIdentity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	execution, err := first.Start(ctx, domainaction.DoorRequest{RequestID: "mismatched-bundle", DoorID: "front", Desired: gateway.LockUnlocked})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanVersion = "1-mutated"
		return nil
	})
	if err != nil {
		t.Fatalf("inject version mismatch: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	_, err = Open(cfg, Dependencies{Door: controller})
	if !errors.Is(err, ErrExecutionBundleMismatch) {
		t.Fatalf("reopen error=%v want ErrExecutionBundleMismatch", err)
	}
}

func TestDoorTerminalHistoricalIdentityDoesNotBlockOpen(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockUnlocked})
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	execution, err := first.Start(ctx, domainaction.DoorRequest{RequestID: "terminal-history", DoorID: "front", Desired: gateway.LockLocked})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusCompleted {
		t.Fatalf("status=%s want=%s", execution.Status, adgo.StatusCompleted)
	}
	_, err = first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanVersion = "retired-terminal-format"
		current.PlanDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		return nil
	})
	if err != nil {
		t.Fatalf("inject terminal identity: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	second, err := Open(cfg, Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("terminal history blocked reopen: %v", err)
	}
	defer second.Close()
	loaded, err := second.production.Store.Load(ctx, execution.ID)
	if err != nil {
		t.Fatalf("load terminal history: %v", err)
	}
	if loaded.Status != adgo.StatusCompleted || loaded.PlanVersion != "retired-terminal-format" {
		t.Fatalf("terminal history changed: status=%s version=%s", loaded.Status, loaded.PlanVersion)
	}
}

func TestDoorRetainedV1CoexistsWithDistinctFutureActiveBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	deps := Dependencies{Door: controller}
	cfg := DefaultConfig(root)

	first, err := Open(cfg, deps)
	if err != nil {
		t.Fatalf("open v1: %v", err)
	}
	oldRequest := domainaction.DoorRequest{RequestID: "retained-v1", DoorID: "front", Desired: gateway.LockUnlocked}
	old, err := first.Start(ctx, oldRequest)
	if err != nil {
		t.Fatalf("start v1: %v", err)
	}
	old, err = first.Drive(ctx, old.ID)
	if err != nil {
		t.Fatalf("drive v1: %v", err)
	}
	if old.Status != adgo.StatusHuman || old.PlanDigest != doorV1PlanDigest {
		t.Fatalf("v1 wait status=%s digest=%s", old.Status, old.PlanDigest)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close v1: %v", err)
	}

	retained, err := doorV1BundleSpec(deps)
	if err != nil {
		t.Fatalf("retained v1 spec: %v", err)
	}
	futurePlan, err := compilePlanVersion("test-future-2")
	if err != nil {
		t.Fatalf("compile future test plan: %v", err)
	}
	future := bundleSpec{
		plan:     futurePlan,
		registry: newRegistryV1(deps),
		bindings: cloneBindings(retained.bindings),
	}
	second, err := openWithBundleSpecs(cfg, future, []bundleSpec{retained, future})
	if err != nil {
		t.Fatalf("open retained-v1 + future-active: %v", err)
	}
	defer second.Close()
	if second.bundles.active.plan.Digest != futurePlan.Digest || futurePlan.Digest == doorV1PlanDigest {
		t.Fatalf("active bundle identity=%s future=%s v1=%s", second.bundles.active.plan.Digest, futurePlan.Digest, doorV1PlanDigest)
	}
	old, err = second.ResolveUnlockApproval(ctx, old.ID, domainaction.ApprovalApprove, "owner", "retained v1")
	if err != nil {
		t.Fatalf("resolve retained v1: %v", err)
	}
	if old.Status != adgo.StatusCompleted || old.PlanDigest != doorV1PlanDigest {
		t.Fatalf("retained v1 completion status=%s digest=%s", old.Status, old.PlanDigest)
	}

	fresh, err := second.Start(ctx, domainaction.DoorRequest{RequestID: "future-active", DoorID: "front", Desired: gateway.LockLocked})
	if err != nil {
		t.Fatalf("start future active: %v", err)
	}
	if fresh.PlanVersion != "test-future-2" || fresh.PlanDigest != futurePlan.Digest {
		t.Fatalf("fresh identity=%s/%s want=%s/%s", fresh.PlanVersion, fresh.PlanDigest, futurePlan.Version, futurePlan.Digest)
	}
	fresh, err = second.Drive(ctx, fresh.ID)
	if err != nil {
		t.Fatalf("drive future active: %v", err)
	}
	if fresh.Status != adgo.StatusCompleted {
		t.Fatalf("future active status=%s", fresh.Status)
	}
}
