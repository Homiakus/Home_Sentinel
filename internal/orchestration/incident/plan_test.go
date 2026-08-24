package incident

import (
	"context"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestPlanCompiles(t *testing.T) {
	plan, err := CompilePlan()
	if err != nil {
		t.Fatalf("compile plan: %v", err)
	}
	if plan.ID != PlanID || plan.Version != PlanVersion || plan.Digest == "" {
		t.Fatalf("unexpected plan identity: %#v", plan)
	}
}

func TestIncidentWaitResumeAndStartDedup(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	config := Config{
		Production:        adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:          "test-worker",
		WorkerConcurrency: 2,
	}
	service, err := Open(config, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open service: %v", err)
	}
	defer service.Close()

	trigger := domainincident.Trigger{
		EventID: "evt-1", SourceID: "front", Kind: "vision.person.detected.v1",
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
	if waiting.Status != adgo.StatusWaiting {
		t.Fatalf("expected waiting status, got %s", waiting.Status)
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

	// Same durable event ID is deduplicated by ADGO. It must not create another
	// notification or reopen the completed execution.
	completedAgain, err := service.OwnerResponse(ctx, first.ID, "owner-response-1", map[string]any{"decision": "ack"})
	if err != nil {
		t.Fatalf("duplicate owner response: %v", err)
	}
	if completedAgain.Status != adgo.StatusCompleted || notifier.Applied != 1 {
		t.Fatalf("duplicate signal changed semantics: status=%s applied=%d", completedAgain.Status, notifier.Applied)
	}
}
