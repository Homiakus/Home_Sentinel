package incident

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	riskpolicy "github.com/Homiakus/Home_Sentinel/internal/policy/risk"
	"github.com/Homiakus/axiom/adgo"
)

func memoryService(t *testing.T, notifier *gatewayfake.Notifier) *Service {
	t.Helper()
	service, err := Open(Config{
		Production:        adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:          "test-worker",
		WorkerConcurrency: 2,
	}, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestPlanCompiles(t *testing.T) {
	plan, err := CompilePlan()
	if err != nil {
		t.Fatalf("compile plan: %v", err)
	}
	if plan.ID != PlanID || plan.Version != PlanVersion || plan.Digest == "" {
		t.Fatalf("unexpected plan identity: %#v", plan)
	}
}

func TestLowRiskArchivesWithoutNotification(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service := memoryService(t, notifier)
	trigger := domainincident.Trigger{
		EventID: "evt-low", SourceID: "yard", Kind: "motion.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.20,
	}
	execution, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusCompleted || notifier.Applied != 0 {
		t.Fatalf("low risk routing failed: status=%s notifications=%d", execution.Status, notifier.Applied)
	}
}

func TestMediumRiskWaitResumeAndStartDedup(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service := memoryService(t, notifier)
	trigger := domainincident.Trigger{
		EventID: "evt-medium", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.96,
	}
	first, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	duplicate, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("duplicate start: %v", err)
	}
	if first.ID != duplicate.ID {
		t.Fatalf("StartOrLoad lost idempotency: %q != %q", first.ID, duplicate.ID)
	}

	waiting, err := service.Drive(ctx, first.ID)
	if err != nil {
		t.Fatalf("drive to wait: %v", err)
	}
	if waiting.Status != adgo.StatusWaiting || waiting.WaitingFor[NodeAwaitAck] != OwnerResponseEvent {
		t.Fatalf("expected owner ack wait, status=%s waiting=%v", waiting.Status, waiting.WaitingFor)
	}
	if notifier.Applied != 1 {
		t.Fatalf("notification applied %d times", notifier.Applied)
	}

	completed, err := service.OwnerResponse(ctx, first.ID, "owner-response-1", map[string]any{"decision": "ack"})
	if err != nil {
		t.Fatalf("owner response: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("expected completed status, got %s; failure=%s", completed.Status, completed.Failure)
	}

	completedAgain, err := service.OwnerResponse(ctx, first.ID, "owner-response-1", map[string]any{"decision": "ack"})
	if err != nil {
		t.Fatalf("duplicate owner response: %v", err)
	}
	if completedAgain.Status != adgo.StatusCompleted || notifier.Applied != 1 {
		t.Fatalf("duplicate signal changed semantics: status=%s applied=%d", completedAgain.Status, notifier.Applied)
	}
}

func TestHighRiskRequiresDurableHumanDecision(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service := memoryService(t, notifier)
	trigger := domainincident.Trigger{
		EventID: "evt-high", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.95,
		Context: domainincident.SecurityContext{
			AlarmMode: "away", Identity: domainincident.IdentityUnknown,
			EntryActive: true, DwellSeconds: 90, CrossCameraMatches: 2,
		},
	}
	execution, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeHumanDecision] != OwnerDecisionEvent {
		t.Fatalf("expected durable human wait: status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}
	if notifier.Applied != 1 {
		t.Fatalf("expected one high-risk notification, got %d", notifier.Applied)
	}

	var assessment riskpolicy.Assessment
	if err := json.Unmarshal(execution.Data["risk_assessment"], &assessment); err != nil {
		t.Fatalf("decode assessment: %v", err)
	}
	if assessment.Risk != domainincident.RiskCritical || len(assessment.Contributions) == 0 {
		t.Fatalf("unexpected explainable assessment: %#v", assessment)
	}

	completed, err := service.ResolveOwnerDecision(
		ctx, execution.ID, domainincident.DecisionApprove,
		"owner-1", "verified visitor", map[string]any{"source": "mobile"},
	)
	if err != nil {
		t.Fatalf("resolve human: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("expected completed after approval, got %s", completed.Status)
	}
	if _, ok := completed.Data["human:"+NodeHumanDecision+":payload"]; !ok {
		t.Fatal("human payload was not durably recorded")
	}
}

func TestHighRiskRejectHasExplicitBranch(t *testing.T) {
	ctx := context.Background()
	service := memoryService(t, gatewayfake.NewNotifier())
	trigger := domainincident.Trigger{
		EventID: "evt-reject", SourceID: "door", Kind: "person",
		OccurredAt: time.Now().UTC(), Confidence: 1,
		Context: domainincident.SecurityContext{
			AlarmMode: "away", Identity: domainincident.IdentityUnknown, EntryActive: true,
		},
	}
	execution, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusHuman {
		t.Fatalf("expected human status, got %s", execution.Status)
	}
	completed, err := service.ResolveOwnerDecision(ctx, execution.ID, domainincident.DecisionReject, "owner", "not authorized", nil)
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || completed.Nodes[NodeArchiveRejected].Status != adgo.NodeCompleted {
		t.Fatalf("rejected branch not archived: status=%s node=%+v", completed.Status, completed.Nodes[NodeArchiveRejected])
	}
}
