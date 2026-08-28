package incident

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	"github.com/Homiakus/axiom/adgo"
)

type realCallbackAuthority struct {
	ring     *callback.Keyring
	acceptor *callback.Acceptor
}

func newRealCallbackAuthority(t *testing.T) *realCallbackAuthority {
	t.Helper()
	key := []byte(strings.Repeat("r", callback.MinKeyBytes))
	ring, err := callback.NewKeyring("active", map[string][]byte{"active": key}, callback.DefaultOptions)
	if err != nil {
		t.Fatalf("new callback keyring: %v", err)
	}
	replay, err := callback.NewReplayGuard(128)
	if err != nil {
		t.Fatalf("new replay guard: %v", err)
	}
	acceptor, err := callback.NewAcceptor(ring, replay)
	if err != nil {
		t.Fatalf("new callback acceptor: %v", err)
	}
	return &realCallbackAuthority{ring: ring, acceptor: acceptor}
}

func (a *realCallbackAuthority) Accept(token string, expected callback.Binding) (callback.Claims, error) {
	return a.acceptor.Accept(token, expected)
}

func (a *realCallbackAuthority) sign(t *testing.T, claims callback.Claims) string {
	t.Helper()
	token, err := a.ring.Sign(claims)
	if err != nil {
		t.Fatalf("sign callback claims: %v", err)
	}
	return token
}

func durableCallbackIngress(authority CallbackAuthority, role auth.Role, audit *callbackAuditFake, workflow CallbackWorkflow) *CallbackIngress {
	id := testOperatorID
	if role == auth.RoleAdmin {
		id = testAdminID
	}
	return &CallbackIngress{
		Security: authority,
		Users:    &callbackUsersFake{users: map[string]auth.User{id: {ID: id, Role: role}}},
		Audit:    audit,
		Workflow: workflow,
	}
}

func TestCallbackMediumRiskDuplicateAfterProcessRestartIsDurablyIdempotent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	notifier := gatewayfake.NewNotifier()
	cfg := DefaultConfig(root)
	cfg.WorkerID = "callback-medium-worker"
	cfg.WorkerConcurrency = 1

	first, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	trigger := domainincident.Trigger{
		EventID: "evt-callback-medium-restart", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.96,
	}
	execution, err := first.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to wait: %v", err)
	}
	if execution.Status != adgo.StatusWaiting || execution.WaitingFor[NodeAwaitAck] != OwnerResponseEvent {
		t.Fatalf("expected medium wait, status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}

	authority1 := newRealCallbackAuthority(t)
	eventID := "owner-response-restart-1"
	token := authority1.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeAwaitAck,
		EventID:     eventID,
		Action:      OwnerResponseEvent,
		Subject:     testOperatorID,
		Nonce:       "medium-restart-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	audit := &callbackAuditFake{}
	ingress1 := durableCallbackIngress(authority1, auth.RoleOperator, audit, first)
	completed, err := ingress1.OwnerResponse(ctx, token, execution.ID, eventID, map[string]any{"decision": "ack"}, CallbackMeta{})
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	if completed.Status != adgo.StatusCompleted || notifier.Applied != 1 {
		t.Fatalf("first callback status=%s notifications=%d", completed.Status, notifier.Applied)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	second, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("reopen service: %v", err)
	}
	defer second.Close()
	// A fresh acceptor models a process restart: its in-memory replay filter has
	// never seen the token, so durable ADGO SeenEvents must be the final dedupe.
	authority2 := newRealCallbackAuthority(t)
	ingress2 := durableCallbackIngress(authority2, auth.RoleOperator, audit, second)
	completedAgain, err := ingress2.OwnerResponse(ctx, token, execution.ID, eventID, map[string]any{"decision": "ack"}, CallbackMeta{})
	if err != nil {
		t.Fatalf("same callback after restart: %v", err)
	}
	if completedAgain.Status != adgo.StatusCompleted || notifier.Applied != 1 {
		t.Fatalf("restart duplicate changed semantics: status=%s notifications=%d", completedAgain.Status, notifier.Applied)
	}
	if len(audit.entries) != 2 {
		t.Fatalf("authorization audit entries=%d want=2 deliveries", len(audit.entries))
	}
}

func TestCallbackHighRiskSecondResolutionAfterRestartIsStaleAndCannotMutateDecision(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	notifier := gatewayfake.NewNotifier()
	cfg := DefaultConfig(root)
	cfg.WorkerID = "callback-high-worker"
	cfg.WorkerConcurrency = 1

	first, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	trigger := domainincident.Trigger{
		EventID: "evt-callback-high-restart", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.95,
		Context: domainincident.SecurityContext{
			AlarmMode: "away", Identity: domainincident.IdentityUnknown,
			EntryActive: true, DwellSeconds: 90, CrossCameraMatches: 2,
		},
	}
	execution, err := first.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive to human wait: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeHumanDecision] != OwnerDecisionEvent {
		t.Fatalf("expected human wait, status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}

	authority1 := newRealCallbackAuthority(t)
	eventID := "owner-decision-restart-1"
	token := authority1.sign(t, callback.Claims{
		ExecutionID: execution.ID,
		NodeID:      NodeHumanDecision,
		EventID:     eventID,
		Action:      OwnerDecisionEvent,
		Subject:     testAdminID,
		Nonce:       "high-restart-nonce",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Unix(),
	})
	audit := &callbackAuditFake{}
	ingress1 := durableCallbackIngress(authority1, auth.RoleAdmin, audit, first)
	completed, err := ingress1.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionApprove, "verified", map[string]any{"source": "callback"}, CallbackMeta{})
	if err != nil {
		t.Fatalf("first high-risk callback: %v", err)
	}
	if completed.Status != adgo.StatusCompleted {
		t.Fatalf("first high-risk callback status=%s", completed.Status)
	}
	payloadKey := "human:" + NodeHumanDecision + ":payload"
	originalPayload := append([]byte(nil), completed.Data[payloadKey]...)
	if len(originalPayload) == 0 {
		t.Fatal("first human resolution payload was not persisted")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	second, err := Open(cfg, Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("reopen service: %v", err)
	}
	defer second.Close()
	authority2 := newRealCallbackAuthority(t)
	ingress2 := durableCallbackIngress(authority2, auth.RoleAdmin, audit, second)
	_, err = ingress2.ResolveOwnerDecision(ctx, token, execution.ID, eventID, domainincident.DecisionReject, "changed after restart", map[string]any{"source": "retry"}, CallbackMeta{})
	if !errors.Is(err, adgo.ErrStaleTask) {
		t.Fatalf("second human resolution error=%v want ErrStaleTask", err)
	}
	loaded, err := second.Get(ctx, execution.ID)
	if err != nil {
		t.Fatalf("load after stale retry: %v", err)
	}
	if loaded.Status != adgo.StatusCompleted {
		t.Fatalf("stale retry changed status to %s", loaded.Status)
	}
	if !bytes.Equal(loaded.Data[payloadKey], originalPayload) {
		t.Fatalf("stale retry changed durable human payload: before=%s after=%s", originalPayload, loaded.Data[payloadKey])
	}
	if loaded.Metrics.HumanInterventions != 1 {
		t.Fatalf("human interventions=%d want=1", loaded.Metrics.HumanInterventions)
	}
}
