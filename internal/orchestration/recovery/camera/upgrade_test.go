package camera

import (
	"context"
	"errors"
	"testing"

	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestCameraV1PlanIdentityIsGolden(t *testing.T) {
	plan, err := CompilePlan()
	if err != nil {
		t.Fatalf("CompilePlan: %v", err)
	}
	if plan.ID != PlanID || plan.Version != PlanVersion || plan.Digest != cameraV1PlanDigest {
		t.Fatalf(
			"camera v1 identity drifted: got=%s/%s/%s want=%s/%s/%s",
			plan.ID, plan.Version, plan.Digest,
			PlanID, PlanVersion, cameraV1PlanDigest,
		)
	}
}

func TestCameraPebbleReopenPreservesOperatorBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": false},
		map[string]bool{"front": false},
	)
	cfg := DefaultConfig(root)

	first, err := Open(cfg, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	request := domainrecovery.CameraRequest{RequestID: "upgrade-operator", CameraID: "front"}
	execution, err := first.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to operator: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeOperator] != OperatorEvent {
		t.Fatalf("operator wait status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}
	persistedDigest := execution.PlanDigest
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	controller.SetNetwork("front", true)
	second, err := Open(cfg, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	completed, err := second.ResolveOperator(ctx, execution.ID, domainrecovery.OperatorRetry, "operator", "network restored after restart")
	if err != nil {
		t.Fatalf("resolve operator after restart: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || completed.PlanDigest != persistedDigest {
		t.Fatalf("completed status=%s digest=%s want digest=%s", completed.Status, completed.PlanDigest, persistedDigest)
	}
	if controller.ReconnectCalls != 1 {
		t.Fatalf("reconnect calls=%d want=1", controller.ReconnectCalls)
	}
}

func TestCameraStartRejectsMismatchedExistingNonTerminalBundle(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": false},
		map[string]bool{"front": false},
	)
	service := openMemory(t, controller)
	request := domainrecovery.CameraRequest{RequestID: "existing-mismatch", CameraID: "front"}
	execution, err := service.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = service.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanVersion = "1-mutated"
		return nil
	})
	if err != nil {
		t.Fatalf("inject version mismatch: %v", err)
	}
	_, err = service.Start(ctx, request)
	if !errors.Is(err, ErrExecutionBundleMismatch) {
		t.Fatalf("duplicate Start error=%v want ErrExecutionBundleMismatch", err)
	}
}

func TestCameraOpenRejectsUnknownNonTerminalBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": false},
		map[string]bool{"front": false},
	)
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	execution, err := first.Start(ctx, domainrecovery.CameraRequest{RequestID: "unknown-bundle", CameraID: "front"})
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
	if _, err := first.Get(ctx, execution.ID); !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("Get error=%v want ErrUnknownExecutionBundle", err)
	}
	serveRuntimeEntered := errors.New("camera test: Serve entered host runtime after failed preflight")
	first.serveHost = func(context.Context, adgo.WorkerSpec) error { return serveRuntimeEntered }
	if err := first.Serve(ctx); !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("Serve preflight error=%v want ErrUnknownExecutionBundle", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	_, err = Open(cfg, Dependencies{Controller: controller})
	if !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("reopen error=%v want ErrUnknownExecutionBundle", err)
	}
}

func TestCameraOpenRejectsMismatchedNonTerminalIdentity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": false},
		map[string]bool{"front": false},
	)
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	execution, err := first.Start(ctx, domainrecovery.CameraRequest{RequestID: "mismatched-bundle", CameraID: "front"})
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

	_, err = Open(cfg, Dependencies{Controller: controller})
	if !errors.Is(err, ErrExecutionBundleMismatch) {
		t.Fatalf("reopen error=%v want ErrExecutionBundleMismatch", err)
	}
}

func TestCameraTerminalHistoricalIdentityRemainsReadable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": true},
		map[string]bool{"front": true},
	)
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	execution, err := first.Start(ctx, domainrecovery.CameraRequest{RequestID: "terminal-history", CameraID: "front"})
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
	loaded, err := first.Get(ctx, execution.ID)
	if err != nil {
		t.Fatalf("Get terminal history: %v", err)
	}
	if loaded.Status != adgo.StatusCompleted || loaded.PlanVersion != "retired-terminal-format" {
		t.Fatalf("terminal history changed: status=%s version=%s", loaded.Status, loaded.PlanVersion)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	second, err := Open(cfg, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("terminal history blocked reopen: %v", err)
	}
	defer second.Close()
	loaded, err = second.Get(ctx, execution.ID)
	if err != nil {
		t.Fatalf("Get terminal after reopen: %v", err)
	}
	if loaded.PlanVersion != "retired-terminal-format" {
		t.Fatalf("terminal version=%s", loaded.PlanVersion)
	}
}

func TestCameraRetainedV1CoexistsWithDistinctFutureActiveBundle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": false},
		map[string]bool{"front": false},
	)
	deps := Dependencies{Controller: controller}
	cfg := DefaultConfig(root)

	first, err := Open(cfg, deps)
	if err != nil {
		t.Fatalf("open v1: %v", err)
	}
	old, err := first.Start(ctx, domainrecovery.CameraRequest{RequestID: "retained-v1", CameraID: "front"})
	if err != nil {
		t.Fatalf("start v1: %v", err)
	}
	old, err = first.Drive(ctx, old.ID)
	if err != nil {
		t.Fatalf("drive v1: %v", err)
	}
	if old.Status != adgo.StatusHuman || old.PlanDigest != cameraV1PlanDigest {
		t.Fatalf("v1 wait status=%s digest=%s", old.Status, old.PlanDigest)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close v1: %v", err)
	}

	retained, err := cameraV1BundleSpec(deps)
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
		bindings: retained.bindings,
	}
	second, err := openWithBundleSpecs(cfg, future, []bundleSpec{retained, future})
	if err != nil {
		t.Fatalf("open retained-v1 + future-active: %v", err)
	}
	defer second.Close()
	if second.bundles.active.plan.Digest != futurePlan.Digest || futurePlan.Digest == cameraV1PlanDigest {
		t.Fatalf("active bundle identity=%s future=%s v1=%s", second.bundles.active.plan.Digest, futurePlan.Digest, cameraV1PlanDigest)
	}

	controller.SetNetwork("front", true)
	old, err = second.ResolveOperator(ctx, old.ID, domainrecovery.OperatorRetry, "operator", "retained v1")
	if err != nil {
		t.Fatalf("resolve retained v1: %v", err)
	}
	if old.Status != adgo.StatusCompleted || old.PlanDigest != cameraV1PlanDigest || controller.ReconnectCalls != 1 {
		t.Fatalf("retained v1 completion status=%s digest=%s reconnects=%d", old.Status, old.PlanDigest, controller.ReconnectCalls)
	}

	fresh, err := second.Start(ctx, domainrecovery.CameraRequest{RequestID: "future-active", CameraID: "front"})
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
	if fresh.Status != adgo.StatusCompleted || controller.ReconnectCalls != 1 {
		t.Fatalf("future active status=%s reconnects=%d", fresh.Status, controller.ReconnectCalls)
	}
}
