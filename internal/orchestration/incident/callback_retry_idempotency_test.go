package incident

import (
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	"github.com/Homiakus/axiom/adgo"
)

func TestCallbackHighRiskRetryDuplicateDoesNotResolveNewHumanWaitTwice(t *testing.T) {
	ctx := t.Context()
	service := memoryService(t, gatewayfake.NewNotifier())
	trigger := domainincident.Trigger{
		EventID: "evt-high-retry-idempotent", SourceID: "front", Kind: "vision.person.detected.v1",
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

	authority := newRealCallbackAuthority(t)
	eventID := "owner-decision-retry-1"
	token := authority.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeHumanDecision,
		EventID:     eventID,
		Action:      OwnerDecisionEvent,
		Subject:     testAdminID,
		Nonce:       "high-retry-idempotent-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	ingress := durableCallbackIngress(authority, auth.RoleAdmin, &callbackAuditFake{}, service)
	payload := map[string]any{"review": "retry"}

	first, err := ingress.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionRetry, "review again", payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("first retry: %v", err)
	}
	if first.Status != adgo.StatusHuman || first.WaitingFor[NodeHumanDecision] != OwnerDecisionEvent {
		t.Fatalf("first retry did not return to human wait: status=%s waiting=%v", first.Status, first.WaitingFor)
	}
	firstRetryHistory := countHistoryType(first, "human_retry")
	if firstRetryHistory != 1 {
		t.Fatalf("first human_retry history=%d want=1", firstRetryHistory)
	}

	second, err := ingress.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionRetry, "review again", payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("exact retry replay: %v", err)
	}
	if second.Status != adgo.StatusHuman || second.WaitingFor[NodeHumanDecision] != OwnerDecisionEvent {
		t.Fatalf("replayed retry changed wait: status=%s waiting=%v", second.Status, second.WaitingFor)
	}
	if got := countHistoryType(second, "human_retry"); got != firstRetryHistory {
		t.Fatalf("duplicate retry applied twice: history=%d want=%d", got, firstRetryHistory)
	}

	// A different signed event is a new human command and must not be blocked by
	// the previous retry receipt. It can resolve the fresh wait exactly once.
	secondEventID := "owner-decision-retry-2"
	secondToken := authority.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeHumanDecision,
		EventID:     secondEventID,
		Action:      OwnerDecisionEvent,
		Subject:     testAdminID,
		Nonce:       "high-retry-followup-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	completed, err := ingress.ResolveOwnerDecision(ctx, secondToken, execution.ID, secondEventID, domainincident.DecisionApprove, "review complete", nil, CallbackMeta{})
	if err != nil {
		t.Fatalf("follow-up decision: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("follow-up status=%s want completed", completed.Status)
	}
}

func countHistoryType(execution *adgo.Execution, eventType string) int {
	count := 0
	for _, entry := range execution.History {
		if entry.Type == eventType {
			count++
		}
	}
	return count
}
