package incident

import (
	"errors"
	"strings"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestFinishStaleCallbackDecisionCoversConvergedAndConflictPaths(t *testing.T) {
	ctx := t.Context()
	service := memoryService(t, gatewayfake.NewNotifier())
	trigger := domainincident.Trigger{
		EventID: "evt-stale-finish", SourceID: "front", Kind: "vision.person.detected.v1",
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
		t.Fatalf("drive to human wait: %v", err)
	}
	if execution.Status != adgo.StatusHuman {
		t.Fatalf("status=%s want human", execution.Status)
	}

	eventID := "owner-decision-stale-finish-1"
	reason := "verified"
	payload := map[string]any{"source": "test"}
	envelope, err := newCallbackDecisionEnvelope(eventID, testAdminID, domainincident.DecisionApprove, reason, payload)
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	completed, err := service.ResolveOwnerCallbackDecision(
		ctx,
		execution.ID,
		eventID,
		domainincident.DecisionApprove,
		testAdminID,
		reason,
		payload,
	)
	if err != nil {
		t.Fatalf("resolve callback decision: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("completed status=%s", completed.Status)
	}

	converged, err := service.finishStaleCallbackDecision(ctx, execution.ID, eventID, envelope.Receipt, adgo.ErrStaleTask)
	if err != nil {
		t.Fatalf("finish exact stale callback: %v", err)
	}
	if converged.Status != adgo.StatusCompleted || converged.Metrics.HumanInterventions != 1 {
		t.Fatalf("converged execution changed: status=%s interventions=%d", converged.Status, converged.Metrics.HumanInterventions)
	}

	conflicting := envelope.Receipt
	conflicting.Digest = strings.Repeat("0", 64)
	_, err = service.finishStaleCallbackDecision(ctx, execution.ID, eventID, conflicting, adgo.ErrStaleTask)
	if !errors.Is(err, ErrCallbackDecisionConflict) || !errors.Is(err, adgo.ErrStaleTask) {
		t.Fatalf("conflicting stale finish error=%v", err)
	}
}
