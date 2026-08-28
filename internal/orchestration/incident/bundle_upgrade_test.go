package incident

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	"github.com/Homiakus/axiom/adgo"
)

func (f *callbackWorkflowFake) OwnerResponseBindingNode(context.Context, string) (string, error) {
	return NodeAwaitAck, nil
}

func (f *callbackWorkflowFake) OwnerDecisionBindingNode(context.Context, string) (string, error) {
	return NodeHumanDecision, nil
}

func legacyTrigger(eventID string) domainincident.Trigger {
	return domainincident.Trigger{
		EventID: eventID, SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.96,
	}
}

func startLegacyV1(t *testing.T, service *Service, trigger domainincident.Trigger) *adgo.Execution {
	t.Helper()
	bundle, err := service.bundles.bundleForVersion(planVersionV1)
	if err != nil {
		t.Fatalf("legacy bundle: %v", err)
	}
	execution, err := bundle.engine.StartOrLoad(
		context.Background(),
		domainincident.ExecutionID(trigger),
		map[string]any{"trigger": trigger},
		adgo.BudgetLimit{},
	)
	if err != nil {
		t.Fatalf("start legacy execution: %v", err)
	}
	return execution
}

func TestIncidentBundleCatalogPinsHistoricalV1Identity(t *testing.T) {
	catalog, err := newBundleCatalog(Dependencies{Notifier: gatewayfake.NewNotifier()})
	if err != nil {
		t.Fatalf("new bundle catalog: %v", err)
	}
	legacy, err := catalog.bundleForVersion(planVersionV1)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.plan.ID != PlanID || legacy.plan.Version != planVersionV1 || legacy.plan.Digest != legacyV1PlanDigest {
		t.Fatalf("unexpected v1 identity: id=%s version=%s digest=%s", legacy.plan.ID, legacy.plan.Version, legacy.plan.Digest)
	}
	if catalog.active.plan.Version != PlanVersion {
		t.Fatalf("active plan version=%s want=%s", catalog.active.plan.Version, PlanVersion)
	}
	if catalog.active.plan.Digest == legacy.plan.Digest {
		t.Fatal("v1 and v2 plan digests unexpectedly match")
	}
	if legacy.bindings.ownerResponseNode != nodeAwaitV1 || legacy.bindings.ownerDecisionNode != "" {
		t.Fatalf("legacy bindings=%+v", legacy.bindings)
	}
}

func TestIncidentServiceRoutesV1AndV2ByPersistedDigest(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service := memoryService(t, notifier)

	legacy := startLegacyV1(t, service, legacyTrigger("evt-bundle-v1"))
	legacy, err := service.Drive(ctx, legacy.ID)
	if err != nil {
		t.Fatalf("drive v1: %v", err)
	}
	if legacy.PlanVersion != planVersionV1 || legacy.Status != adgo.StatusWaiting || legacy.WaitingFor[nodeAwaitV1] != OwnerResponseEvent {
		t.Fatalf("v1 routing mismatch: version=%s status=%s waiting=%v", legacy.PlanVersion, legacy.Status, legacy.WaitingFor)
	}
	var score float64
	if err := json.Unmarshal(legacy.Data["risk_score"], &score); err != nil {
		t.Fatalf("decode v1 risk score: %v", err)
	}
	if math.Abs(score-0.828) > 1e-9 {
		t.Fatalf("v1 risk score=%f want=0.828", score)
	}
	if _, ok := legacy.Data["risk_assessment"]; ok {
		t.Fatal("v1 execution was reinterpreted with v2 risk policy")
	}
	if _, err := service.Explain(ctx, legacy.ID, nodeAwaitV1); err != nil {
		t.Fatalf("explain v1: %v", err)
	}
	if _, err := service.Diagnostics(ctx, legacy.ID); err != nil {
		t.Fatalf("diagnostics v1: %v", err)
	}

	active, err := service.Start(ctx, legacyTrigger("evt-bundle-v2"))
	if err != nil {
		t.Fatalf("start v2: %v", err)
	}
	active, err = service.Drive(ctx, active.ID)
	if err != nil {
		t.Fatalf("drive v2: %v", err)
	}
	if active.PlanVersion != PlanVersion || active.Status != adgo.StatusWaiting || active.WaitingFor[NodeAwaitAck] != OwnerResponseEvent {
		t.Fatalf("v2 routing mismatch: version=%s status=%s waiting=%v", active.PlanVersion, active.Status, active.WaitingFor)
	}
	if notifier.Applied != 2 {
		t.Fatalf("notifications=%d want=2", notifier.Applied)
	}
}

func TestIncidentV1PebbleReopenUsesHistoricalCallbackBindingAndHandlers(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	notifier := gatewayfake.NewNotifier()
	cfg := DefaultConfig(root)
	cfg.WorkerID = "bundle-upgrade-worker"
	cfg.WorkerConcurrency = 1

	first, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	trigger := legacyTrigger("evt-v1-pebble-upgrade")
	execution := startLegacyV1(t, first, trigger)
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive v1 before reopen: %v", err)
	}
	if execution.Status != adgo.StatusWaiting || execution.WaitingFor[nodeAwaitV1] != OwnerResponseEvent {
		t.Fatalf("v1 wait before reopen: status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}
	if notifier.Applied != 1 {
		t.Fatalf("v1 notification count before reopen=%d", notifier.Applied)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	second, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("reopen with v2 active and v1 retained: %v", err)
	}
	defer second.Close()

	authority := newRealCallbackAuthority(t)
	eventID := "owner-response-v1-upgrade"
	token := authority.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      nodeAwaitV1,
		EventID:     eventID,
		Action:      OwnerResponseEvent,
		Subject:     testOperatorID,
		Nonce:       "v1-upgrade-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	audit := &callbackAuditFake{}
	ingress := durableCallbackIngress(authority, auth.RoleOperator, audit, second)
	completed, err := ingress.OwnerResponse(ctx, token, execution.ID, eventID, map[string]any{"decision": "ack"}, CallbackMeta{})
	if err != nil {
		t.Fatalf("v1 callback after reopen: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || completed.PlanVersion != planVersionV1 {
		t.Fatalf("v1 completion after reopen: status=%s version=%s", completed.Status, completed.PlanVersion)
	}
	var archived bool
	if err := json.Unmarshal(completed.Data["archived"], &archived); err != nil || !archived {
		t.Fatalf("historical v1 archive fact missing: archived=%v err=%v", archived, err)
	}
	if _, ok := completed.Data["risk_assessment"]; ok {
		t.Fatal("reopened v1 execution acquired v2 risk assessment")
	}
	if notifier.Applied != 1 {
		t.Fatalf("notification duplicated across v1 reopen: %d", notifier.Applied)
	}
	if len(audit.entries) != 1 || audit.entries[0].Target != execution.ID+"/"+nodeAwaitV1 {
		t.Fatalf("v1 callback audit binding=%+v", audit.entries)
	}

	active, err := second.Start(ctx, legacyTrigger("evt-v2-after-upgrade"))
	if err != nil {
		t.Fatalf("start active execution after reopen: %v", err)
	}
	if active.PlanVersion != PlanVersion || active.PlanDigest == legacyV1PlanDigest {
		t.Fatalf("new execution did not use v2: version=%s digest=%s", active.PlanVersion, active.PlanDigest)
	}
}

func TestIncidentOpenFailsClosedOnUnknownNonTerminalPlanDigest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := DefaultConfig(root)
	first, err := Open(cfg, Dependencies{Notifier: gatewayfake.NewNotifier()})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	execution, err := first.Start(ctx, legacyTrigger("evt-unknown-digest"))
	if err != nil {
		t.Fatalf("start execution: %v", err)
	}
	_, err = first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanDigest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		return nil
	})
	if err != nil {
		t.Fatalf("inject unknown digest fixture: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	_, err = Open(cfg, Dependencies{Notifier: gatewayfake.NewNotifier()})
	if !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("reopen error=%v want ErrUnknownExecutionBundle", err)
	}
}
