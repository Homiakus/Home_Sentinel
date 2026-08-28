package incident

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	"github.com/Homiakus/axiom/adgo"
)

func TestCallbackMediumRiskSameProcessReplayConvergesOnDurableSignal(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service := memoryService(t, notifier)
	trigger := domainincident.Trigger{
		EventID: "evt-medium-same-process", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.96,
	}
	execution, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to wait: %v", err)
	}
	if execution.Status != adgo.StatusWaiting {
		t.Fatalf("status=%s want waiting", execution.Status)
	}

	authority := newRealCallbackAuthority(t)
	eventID := "owner-response-same-process-1"
	token := authority.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeAwaitAck,
		EventID:     eventID,
		Action:      OwnerResponseEvent,
		Subject:     testOperatorID,
		Nonce:       "medium-same-process-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	audit := &callbackAuditFake{}
	ingress := durableCallbackIngress(authority, auth.RoleOperator, audit, service)
	payload := map[string]any{"decision": "ack"}

	first, err := ingress.OwnerResponse(ctx, token, execution.ID, eventID, payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	second, err := ingress.OwnerResponse(ctx, token, execution.ID, eventID, payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("same-process replay: %v", err)
	}
	if first.Status != adgo.StatusCompleted || second.Status != adgo.StatusCompleted {
		t.Fatalf("statuses first=%s second=%s", first.Status, second.Status)
	}
	if notifier.Applied != 1 {
		t.Fatalf("notification side effect applied %d times", notifier.Applied)
	}
	if len(audit.entries) != 2 {
		t.Fatalf("audit entries=%d want=2", len(audit.entries))
	}
	var details map[string]any
	if err := json.Unmarshal(audit.entries[1].Details, &details); err != nil {
		t.Fatalf("decode replay audit: %v", err)
	}
	if details["reason_code"] != "replay_candidate" || audit.entries[1].Result != "allowed" {
		t.Fatalf("replay audit=%+v details=%v", audit.entries[1], details)
	}
}

func TestCallbackHighRiskSameProcessReplayReturnsSuccessAndChangedInputConflicts(t *testing.T) {
	ctx := context.Background()
	service := memoryService(t, gatewayfake.NewNotifier())
	trigger := domainincident.Trigger{
		EventID: "evt-high-same-process", SourceID: "front", Kind: "vision.person.detected.v1",
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
	eventID := "owner-decision-same-process-1"
	token := authority.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeHumanDecision,
		EventID:     eventID,
		Action:      OwnerDecisionEvent,
		Subject:     testAdminID,
		Nonce:       "high-same-process-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	ingress := durableCallbackIngress(authority, auth.RoleAdmin, &callbackAuditFake{}, service)
	payload := map[string]any{"source": "callback", "review": "verified"}

	first, err := ingress.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionApprove, "verified visitor", payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("first decision: %v", err)
	}
	second, err := ingress.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionApprove, "verified visitor", payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("exact replay: %v", err)
	}
	if first.Status != adgo.StatusCompleted || second.Status != adgo.StatusCompleted {
		t.Fatalf("statuses first=%s second=%s", first.Status, second.Status)
	}
	if second.Metrics.HumanInterventions != 1 {
		t.Fatalf("human interventions=%d want=1", second.Metrics.HumanInterventions)
	}

	_, err = ingress.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionReject, "changed", map[string]any{"source": "changed"}, CallbackMeta{})
	if !errors.Is(err, ErrCallbackDecisionConflict) {
		t.Fatalf("changed replay error=%v want conflict", err)
	}
	if !errors.Is(err, adgo.ErrStaleTask) {
		t.Fatalf("conflict must retain stale-task compatibility: %v", err)
	}
	loaded, err := service.Get(ctx, execution.ID)
	if err != nil {
		t.Fatalf("load after conflict: %v", err)
	}
	if loaded.Status != adgo.StatusCompleted || loaded.Metrics.HumanInterventions != 1 {
		t.Fatalf("conflict mutated execution: status=%s interventions=%d", loaded.Status, loaded.Metrics.HumanInterventions)
	}
}

func TestCallbackHighRiskExactDuplicateAfterPebbleReopenReturnsPriorSuccess(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	notifier := gatewayfake.NewNotifier()
	cfg := DefaultConfig(root)
	cfg.WorkerID = "callback-exact-reopen-worker"
	cfg.WorkerConcurrency = 1

	firstService, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	trigger := domainincident.Trigger{
		EventID: "evt-high-exact-reopen", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.95,
		Context: domainincident.SecurityContext{
			AlarmMode: "away", Identity: domainincident.IdentityUnknown,
			EntryActive: true, DwellSeconds: 90, CrossCameraMatches: 2,
		},
	}
	execution, err := firstService.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = firstService.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to human wait: %v", err)
	}

	authority1 := newRealCallbackAuthority(t)
	eventID := "owner-decision-exact-reopen-1"
	token := authority1.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeHumanDecision,
		EventID:     eventID,
		Action:      OwnerDecisionEvent,
		Subject:     testAdminID,
		Nonce:       "high-exact-reopen-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	payload := map[string]any{"source": "callback", "review": "verified"}
	audit := &callbackAuditFake{}
	ingress1 := durableCallbackIngress(authority1, auth.RoleAdmin, audit, firstService)
	completed, err := ingress1.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionApprove, "verified visitor", payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("first decision: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("first status=%s", completed.Status)
	}
	if err := firstService.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	secondService, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("reopen service: %v", err)
	}
	defer secondService.Close()
	// Fresh replay state models a full process restart. The same signed token is
	// cryptographically valid again, so the durable callback receipt must be the
	// source of truth for exact duplicate reconciliation.
	authority2 := newRealCallbackAuthority(t)
	ingress2 := durableCallbackIngress(authority2, auth.RoleAdmin, audit, secondService)
	completedAgain, err := ingress2.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionApprove, "verified visitor", payload, CallbackMeta{})
	if err != nil {
		t.Fatalf("exact decision after reopen: %v", err)
	}
	if completedAgain.Status != adgo.StatusCompleted || completedAgain.Metrics.HumanInterventions != 1 {
		t.Fatalf("reopen duplicate changed semantics: status=%s interventions=%d", completedAgain.Status, completedAgain.Metrics.HumanInterventions)
	}
	if notifier.Applied != 1 {
		t.Fatalf("notification side effect applied %d times", notifier.Applied)
	}
}
