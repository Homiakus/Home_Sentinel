package incident

import (
	"context"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestIncidentSurvivesPebbleReopen(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	notifier := gatewayfake.NewNotifier()
	config := Config{
		Production: adgo.DefaultProductionConfig(root),
		WorkerID:   "pebble-test-worker", WorkerConcurrency: 1,
	}

	firstService, err := Open(config, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	trigger := domainincident.Trigger{
		EventID: "evt-reopen", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.96,
	}
	execution, err := firstService.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = firstService.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to waiting: %v", err)
	}
	if execution.Status != adgo.StatusWaiting {
		t.Fatalf("expected waiting before reopen, got %s", execution.Status)
	}
	if err := firstService.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	secondService, err := Open(config, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("reopen service: %v", err)
	}
	defer secondService.Close()

	loaded, err := secondService.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("StartOrLoad after reopen: %v", err)
	}
	if loaded.ID != execution.ID || loaded.Status != adgo.StatusWaiting {
		t.Fatalf("durable execution mismatch: id=%s status=%s", loaded.ID, loaded.Status)
	}
	completed, err := secondService.OwnerResponse(ctx, loaded.ID, "owner-after-reopen", nil)
	if err != nil {
		t.Fatalf("resume after reopen: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("expected completed after reopen, got %s", completed.Status)
	}
	if notifier.Applied != 1 {
		t.Fatalf("notification duplicated across reopen: %d", notifier.Applied)
	}
}
