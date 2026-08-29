package fault

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/action/door"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/action/siren"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/recovery/camera"
	"github.com/Homiakus/axiom/adgo"
)

var errInjectedCompletionCrash = errors.New("fault test: provider applied but ADGO completion commit was lost")

const faultLeaseTTL = time.Hour

type completionCrashStore struct {
	adgo.Store

	mu       sync.Mutex
	armed    bool
	tripped  bool
	failures int
}

func (s *completionCrashStore) armOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tripped {
		return
	}
	s.armed = true
}

func (s *completionCrashStore) Commit(
	ctx context.Context,
	executionID string,
	expected uint64,
	mutate func(*adgo.Execution) error,
) (*adgo.Execution, error) {
	s.mu.Lock()
	if s.armed {
		s.armed = false
		s.tripped = true
		s.failures++
		s.mu.Unlock()
		return nil, errInjectedCompletionCrash
	}
	s.mu.Unlock()
	return s.Store.Commit(ctx, executionID, expected, mutate)
}

func (s *completionCrashStore) failureCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failures
}

type observedDoorController struct {
	inner *gatewayfake.DoorController
	arm   func()

	mu   sync.Mutex
	keys []string
}

func (c *observedDoorController) LockState(ctx context.Context, doorID string) (gateway.LockState, error) {
	return c.inner.LockState(ctx, doorID)
}

func (c *observedDoorController) SetLockState(
	ctx context.Context,
	op gateway.Operation,
	doorID string,
	desired gateway.LockState,
) (gateway.EffectResult, error) {
	result, err := c.inner.SetLockState(ctx, op, doorID, desired)
	c.mu.Lock()
	c.keys = append(c.keys, op.IdempotencyKey)
	c.mu.Unlock()
	if err == nil && result.State == gateway.EffectApplied {
		c.arm()
	}
	return result, err
}

func (c *observedDoorController) observedKeys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.keys...)
}

type observedSirenController struct {
	inner *gatewayfake.SirenController
	arm   func()

	mu          sync.Mutex
	enableKeys  []string
	disableKeys []string
}

func (c *observedSirenController) Enabled(ctx context.Context, sirenID string) (bool, error) {
	return c.inner.Enabled(ctx, sirenID)
}

func (c *observedSirenController) SetEnabled(
	ctx context.Context,
	op gateway.Operation,
	sirenID string,
	desired bool,
) (gateway.EffectResult, error) {
	result, err := c.inner.SetEnabled(ctx, op, sirenID, desired)
	c.mu.Lock()
	if desired {
		c.enableKeys = append(c.enableKeys, op.IdempotencyKey)
	} else {
		c.disableKeys = append(c.disableKeys, op.IdempotencyKey)
	}
	c.mu.Unlock()
	if desired && err == nil && result.State == gateway.EffectApplied {
		c.arm()
	}
	return result, err
}

func (c *observedSirenController) observedKeys() (enable, disable []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.enableKeys...), append([]string(nil), c.disableKeys...)
}

type observedCameraController struct {
	inner *gatewayfake.CameraRecoveryController
	arm   func()

	mu   sync.Mutex
	keys []string
}

func (c *observedCameraController) ProbeNetwork(ctx context.Context, cameraID string) (bool, error) {
	return c.inner.ProbeNetwork(ctx, cameraID)
}

func (c *observedCameraController) ProbeStream(ctx context.Context, cameraID string) (bool, error) {
	return c.inner.ProbeStream(ctx, cameraID)
}

func (c *observedCameraController) Reconnect(
	ctx context.Context,
	op gateway.Operation,
	cameraID string,
) (gateway.EffectResult, error) {
	result, err := c.inner.Reconnect(ctx, op, cameraID)
	c.mu.Lock()
	c.keys = append(c.keys, op.IdempotencyKey)
	c.mu.Unlock()
	if err == nil && result.State == gateway.EffectApplied {
		c.arm()
	}
	return result, err
}

func (c *observedCameraController) observedKeys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.keys...)
}

func faultWorker(id string) adgo.WorkerSpec {
	return adgo.WorkerSpec{ID: id, Concurrency: 1, LeaseTTL: faultLeaseTTL}
}

func openPebble(t *testing.T, path string) *adgo.PebbleStore {
	t.Helper()
	store, err := adgo.OpenPebbleStore(path)
	if err != nil {
		t.Fatalf("open pebble: %v", err)
	}
	return store
}

func newEngine(t *testing.T, plan *adgo.Plan, store adgo.Store, registry *adgo.Registry) *adgo.Engine {
	t.Helper()
	engine, err := adgo.NewEngine(plan, store, registry, adgo.WithEngineLeaseTTL(faultLeaseTTL))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine
}

func startAndCrashAfterProvider(
	t *testing.T,
	ctx context.Context,
	engine *adgo.Engine,
	store *completionCrashStore,
	executionID string,
	initial map[string]any,
	expectedNode string,
) (taskID, idempotencyKey string) {
	t.Helper()
	if _, err := engine.StartOrLoad(ctx, executionID, initial, adgo.BudgetLimit{}); err != nil {
		t.Fatalf("start execution: %v", err)
	}
	if _, err := engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("pre-crash")}); !errors.Is(err, errInjectedCompletionCrash) {
		t.Fatalf("RunLocal error=%v want injected completion crash", err)
	}
	if got := store.failureCount(); got != 1 {
		t.Fatalf("completion crash count=%d want=1", got)
	}
	persisted, err := store.Store.Load(ctx, executionID)
	if err != nil {
		t.Fatalf("load after injected crash: %v", err)
	}
	taskID, task := requireSingleActiveTask(t, persisted)
	if task.Status != adgo.TaskRunning {
		t.Fatalf("persisted task status=%s want=%s", task.Status, adgo.TaskRunning)
	}
	if task.NodeID != expectedNode {
		t.Fatalf("persisted task node=%s want=%s", task.NodeID, expectedNode)
	}
	if task.IdempotencyKey == "" {
		t.Fatal("persisted task idempotency key is empty")
	}
	return taskID, task.IdempotencyKey
}

func reopenAndReenqueue(
	t *testing.T,
	ctx context.Context,
	path string,
	plan *adgo.Plan,
	registry *adgo.Registry,
	executionID, crashedTaskID, originalKey, expectedNode string,
) (*adgo.PebbleStore, *adgo.Engine) {
	t.Helper()
	store := openPebble(t, path)
	persisted, err := store.Load(ctx, executionID)
	if err != nil {
		_ = store.Close()
		t.Fatalf("load after reopen: %v", err)
	}
	if _, ok := persisted.ActiveTasks[crashedTaskID]; !ok {
		_ = store.Close()
		t.Fatalf("crashed task %s missing after reopen", crashedTaskID)
	}
	if _, err := store.Commit(ctx, executionID, persisted.Version, func(current *adgo.Execution) error {
		task := current.ActiveTasks[crashedTaskID]
		task.LeaseUntil = time.Unix(1, 0).UTC()
		current.ActiveTasks[crashedTaskID] = task
		return nil
	}); err != nil {
		_ = store.Close()
		t.Fatalf("expire crashed lease: %v", err)
	}

	engine := newEngine(t, plan, store, registry)
	if _, err := engine.Advance(ctx, executionID); err != nil {
		_ = store.Close()
		t.Fatalf("recover and re-enqueue: %v", err)
	}
	recovered, err := store.Load(ctx, executionID)
	if err != nil {
		_ = store.Close()
		t.Fatalf("load recovered execution: %v", err)
	}
	newTaskID, task := requireSingleActiveTask(t, recovered)
	if newTaskID == crashedTaskID {
		_ = store.Close()
		t.Fatalf("lease recovery reused task id %s instead of creating a new fenced attempt", newTaskID)
	}
	if task.Status != adgo.TaskPending {
		_ = store.Close()
		t.Fatalf("recovered task status=%s want=%s", task.Status, adgo.TaskPending)
	}
	if task.NodeID != expectedNode {
		_ = store.Close()
		t.Fatalf("recovered task node=%s want=%s", task.NodeID, expectedNode)
	}
	if task.IdempotencyKey != originalKey {
		_ = store.Close()
		t.Fatalf("redelivery key=%q want original %q", task.IdempotencyKey, originalKey)
	}
	if task.Attempt < 2 {
		_ = store.Close()
		t.Fatalf("recovered attempt=%d want >=2", task.Attempt)
	}
	return store, engine
}

func forceWaitingTimerDue(t *testing.T, ctx context.Context, store adgo.Store, executionID, nodeID string) {
	t.Helper()
	persisted, err := store.Load(ctx, executionID)
	if err != nil {
		t.Fatalf("load waiting timer: %v", err)
	}
	node := persisted.Nodes[nodeID]
	if node == nil {
		t.Fatalf("waiting timer node %q is missing", nodeID)
	}
	if node.Status != adgo.NodeWaiting || node.NotBefore.IsZero() {
		t.Fatalf("timer node %q status=%s notBefore=%s", nodeID, node.Status, node.NotBefore)
	}
	if _, err := store.Commit(ctx, executionID, persisted.Version, func(current *adgo.Execution) error {
		current.Nodes[nodeID].NotBefore = time.Unix(1, 0).UTC()
		return nil
	}); err != nil {
		t.Fatalf("force timer %q due: %v", nodeID, err)
	}
}

func requireSingleActiveTask(t *testing.T, execution *adgo.Execution) (string, adgo.TaskRuntime) {
	t.Helper()
	if len(execution.ActiveTasks) != 1 {
		t.Fatalf("active task count=%d want=1", len(execution.ActiveTasks))
	}
	for id, task := range execution.ActiveTasks {
		return id, task
	}
	t.Fatal("active task map unexpectedly empty")
	return "", adgo.TaskRuntime{}
}

func TestDoorCompletionCommitCrashRedeliversWithoutSecondPhysicalWrite(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "door-pebble")
	base := openPebble(t, path)
	crashStore := &completionCrashStore{Store: base}
	inner := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockUnlocked})
	observed := &observedDoorController{inner: inner, arm: crashStore.armOnce}
	plan, err := door.CompilePlan()
	if err != nil {
		t.Fatalf("compile door: %v", err)
	}
	engine := newEngine(t, plan, crashStore, door.NewRegistry(door.Dependencies{Door: observed}))
	request := domainaction.DoorRequest{RequestID: "crash-lock", DoorID: "front", Desired: gateway.LockLocked}
	executionID := domainaction.DoorExecutionID(request)
	crashedTaskID, originalKey := startAndCrashAfterProvider(
		t, ctx, engine, crashStore, executionID, map[string]any{"request": request}, door.NodeApplyLock,
	)
	if inner.Applied != 1 || inner.Calls != 1 {
		t.Fatalf("pre-restart door physical state applied=%d calls=%d want 1/1", inner.Applied, inner.Calls)
	}
	if keys := observed.observedKeys(); len(keys) != 1 || keys[0] != originalKey {
		t.Fatalf("pre-restart door keys=%v want [%s]", keys, originalKey)
	}
	if err := base.Close(); err != nil {
		t.Fatalf("close crashed door store: %v", err)
	}

	reopened, recoveredEngine := reopenAndReenqueue(
		t, ctx, path, plan, door.NewRegistry(door.Dependencies{Door: observed}),
		executionID, crashedTaskID, originalKey, door.NodeApplyLock,
	)
	defer reopened.Close()
	finished, err := recoveredEngine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("door-recovery")})
	if err != nil {
		t.Fatalf("door recovery RunLocal: %v", err)
	}
	if finished.Status != adgo.StatusCompleted {
		t.Fatalf("door recovery status=%s want=%s", finished.Status, adgo.StatusCompleted)
	}
	state, err := inner.LockState(ctx, "front")
	if err != nil {
		t.Fatalf("read recovered door: %v", err)
	}
	if state != gateway.LockLocked || inner.Applied != 1 || inner.Calls != 1 {
		t.Fatalf("door recovery state=%s applied=%d calls=%d", state, inner.Applied, inner.Calls)
	}
	if keys := observed.observedKeys(); len(keys) != 1 || keys[0] != originalKey {
		t.Fatalf("door provider was reinvoked after recovery: keys=%v", keys)
	}
}

func TestSirenCompletionCommitCrashDoesNotDuplicateEnableAndStillDisables(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "siren-pebble")
	base := openPebble(t, path)
	crashStore := &completionCrashStore{Store: base}
	inner := gatewayfake.NewSirenController(map[string]bool{"yard": false})
	observed := &observedSirenController{inner: inner, arm: crashStore.armOnce}
	plan, err := siren.CompilePlan(time.Hour)
	if err != nil {
		t.Fatalf("compile siren: %v", err)
	}
	engine := newEngine(t, plan, crashStore, siren.NewRegistry(siren.Dependencies{Siren: observed}))
	request := domainaction.SirenRequest{RequestID: "crash-enable", SirenID: "yard"}
	executionID := domainaction.SirenExecutionID(request)
	crashedTaskID, originalKey := startAndCrashAfterProvider(
		t, ctx, engine, crashStore, executionID, map[string]any{"request": request}, siren.NodeEnable,
	)
	enabled, err := inner.Enabled(ctx, "yard")
	if err != nil {
		t.Fatalf("read siren after injected crash: %v", err)
	}
	if !enabled || inner.Applied != 1 {
		t.Fatalf("siren after injected crash enabled=%v applied=%d", enabled, inner.Applied)
	}
	enableKeys, disableKeys := observed.observedKeys()
	if len(enableKeys) != 1 || enableKeys[0] != originalKey || len(disableKeys) != 0 {
		t.Fatalf("pre-restart siren keys enable=%v disable=%v", enableKeys, disableKeys)
	}
	if err := base.Close(); err != nil {
		t.Fatalf("close crashed siren store: %v", err)
	}

	reopened, recoveredEngine := reopenAndReenqueue(
		t, ctx, path, plan, siren.NewRegistry(siren.Dependencies{Siren: observed}),
		executionID, crashedTaskID, originalKey, siren.NodeEnable,
	)
	defer reopened.Close()
	waiting, err := recoveredEngine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("siren-recovery")})
	if err != nil {
		t.Fatalf("siren recovery RunLocal: %v", err)
	}
	if waiting.Status != adgo.StatusWaiting {
		t.Fatalf("siren recovery status=%s want=%s before safety deadline", waiting.Status, adgo.StatusWaiting)
	}
	enabled, err = inner.Enabled(ctx, "yard")
	if err != nil {
		t.Fatalf("read recovered siren before safety deadline: %v", err)
	}
	enableKeys, disableKeys = observed.observedKeys()
	if !enabled {
		t.Fatal("siren disabled before controlled safety deadline")
	}
	if len(enableKeys) != 1 || enableKeys[0] != originalKey {
		t.Fatalf("siren enable was physically reinvoked: keys=%v", enableKeys)
	}
	if len(disableKeys) != 0 || inner.Applied != 1 || inner.Calls != 1 {
		t.Fatalf("unexpected pre-deadline siren disable keys=%v applied=%d calls=%d", disableKeys, inner.Applied, inner.Calls)
	}

	forceWaitingTimerDue(t, ctx, reopened, executionID, siren.NodeSafety)
	finished, err := recoveredEngine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("siren-safety")})
	if err != nil {
		t.Fatalf("siren safety RunLocal: %v", err)
	}
	if finished.Status != adgo.StatusCompleted {
		t.Fatalf("siren safety status=%s want=%s", finished.Status, adgo.StatusCompleted)
	}
	enabled, err = inner.Enabled(ctx, "yard")
	if err != nil {
		t.Fatalf("read recovered siren after safety deadline: %v", err)
	}
	enableKeys, disableKeys = observed.observedKeys()
	if enabled {
		t.Fatal("siren remained enabled after controlled safety deadline")
	}
	if len(enableKeys) != 1 || enableKeys[0] != originalKey {
		t.Fatalf("siren enable changed after safety deadline: keys=%v", enableKeys)
	}
	if len(disableKeys) != 1 {
		t.Fatalf("siren safety disable calls=%d want=1 keys=%v", len(disableKeys), disableKeys)
	}
	if inner.Applied != 2 || inner.Calls != 2 {
		t.Fatalf("siren physical operations applied=%d calls=%d want 2/2 (enable + safety disable)", inner.Applied, inner.Calls)
	}
}

func TestCameraReconnectCompletionCommitCrashRedeliversSameProviderKey(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "camera-pebble")
	base := openPebble(t, path)
	crashStore := &completionCrashStore{Store: base}
	inner := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": true},
		map[string]bool{"front": false},
	)
	observed := &observedCameraController{inner: inner, arm: crashStore.armOnce}
	plan, err := camera.CompilePlan()
	if err != nil {
		t.Fatalf("compile camera: %v", err)
	}
	engine := newEngine(t, plan, crashStore, camera.NewRegistry(camera.Dependencies{Controller: observed}))
	request := domainrecovery.CameraRequest{RequestID: "crash-reconnect", CameraID: "front"}
	executionID := domainrecovery.CameraExecutionID(request)
	crashedTaskID, originalKey := startAndCrashAfterProvider(
		t, ctx, engine, crashStore, executionID, map[string]any{"request": request}, camera.NodeReconnect,
	)
	if inner.ReconnectCalls != 1 {
		t.Fatalf("pre-restart physical reconnects=%d want=1", inner.ReconnectCalls)
	}
	if keys := observed.observedKeys(); len(keys) != 1 || keys[0] != originalKey {
		t.Fatalf("pre-restart camera keys=%v want [%s]", keys, originalKey)
	}
	if err := base.Close(); err != nil {
		t.Fatalf("close crashed camera store: %v", err)
	}

	reopened, recoveredEngine := reopenAndReenqueue(
		t, ctx, path, plan, camera.NewRegistry(camera.Dependencies{Controller: observed}),
		executionID, crashedTaskID, originalKey, camera.NodeReconnect,
	)
	defer reopened.Close()
	finished, err := recoveredEngine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("camera-recovery")})
	if err != nil {
		t.Fatalf("camera recovery RunLocal: %v", err)
	}
	if finished.Status != adgo.StatusCompleted {
		t.Fatalf("camera recovery status=%s want=%s", finished.Status, adgo.StatusCompleted)
	}
	streamOK, err := inner.ProbeStream(ctx, "front")
	if err != nil {
		t.Fatalf("probe recovered stream: %v", err)
	}
	keys := observed.observedKeys()
	if !streamOK || inner.ReconnectCalls != 1 {
		t.Fatalf("camera recovery streamOK=%v physicalReconnects=%d", streamOK, inner.ReconnectCalls)
	}
	if len(keys) != 2 || keys[0] != originalKey || keys[1] != originalKey {
		t.Fatalf("camera provider redelivery keys=%v want [%s %s]", keys, originalKey, originalKey)
	}
}

func TestAmbiguousDoorOutcomeDoesNotBlindRetry(t *testing.T) {
	ctx := context.Background()
	inner := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockUnlocked})
	inner.SetNextAmbiguity(gatewayfake.DoorAmbiguousNotApplied)
	plan, err := door.CompilePlan()
	if err != nil {
		t.Fatalf("compile door: %v", err)
	}
	engine := newEngine(t, plan, adgo.NewMemoryStore(), door.NewRegistry(door.Dependencies{Door: inner}))
	request := domainaction.DoorRequest{RequestID: "ambiguous-lock", DoorID: "front", Desired: gateway.LockLocked}
	executionID := domainaction.DoorExecutionID(request)
	if _, err := engine.StartOrLoad(ctx, executionID, map[string]any{"request": request}, adgo.BudgetLimit{}); err != nil {
		t.Fatalf("start ambiguous door: %v", err)
	}
	waiting, err := engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("ambiguous-door")})
	if err != nil {
		t.Fatalf("drive ambiguous door: %v", err)
	}
	if waiting.Status != adgo.StatusHuman || waiting.WaitingFor[door.NodeApplyLock] != "Reconcile:"+door.NodeApplyLock {
		t.Fatalf("ambiguous door status=%s waiting=%v", waiting.Status, waiting.WaitingFor)
	}
	if inner.Calls != 1 || inner.Applied != 0 {
		t.Fatalf("ambiguous door provider calls=%d applied=%d want 1/0", inner.Calls, inner.Applied)
	}
	again, err := engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: faultWorker("ambiguous-door-again")})
	if err != nil {
		t.Fatalf("redrive unresolved ambiguous door: %v", err)
	}
	if again.Status != adgo.StatusHuman || inner.Calls != 1 || inner.Applied != 0 {
		t.Fatalf("unresolved ambiguity was replayed: status=%s calls=%d applied=%d", again.Status, inner.Calls, inner.Applied)
	}
}
