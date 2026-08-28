package incident

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/authz"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	"github.com/Homiakus/axiom/adgo"
)

const (
	testOperatorID = "usr_00000000000000000000000000"
	testAdminID    = "usr_11111111111111111111111111"
)

type callbackAuthorityFake struct {
	claims callback.Claims
	err    error
	seen   callback.Binding
	calls  int
}

func (f *callbackAuthorityFake) Accept(_ string, expected callback.Binding) (callback.Claims, error) {
	f.calls++
	f.seen = expected
	return f.claims, f.err
}

type callbackUsersFake struct {
	users map[string]auth.User
	calls int
}

func (f *callbackUsersFake) GetByID(_ context.Context, id string) (auth.User, error) {
	f.calls++
	user, ok := f.users[id]
	if !ok {
		return auth.User{}, errors.New("user not found")
	}
	return user, nil
}

type callbackAuditFake struct {
	entries []repository.AuditEntry
	err     error
}

func (f *callbackAuditFake) Append(_ context.Context, entry repository.AuditEntry) (repository.AuditEntry, error) {
	if f.err != nil {
		return repository.AuditEntry{}, f.err
	}
	f.entries = append(f.entries, entry)
	return entry, nil
}

type callbackWorkflowFake struct {
	ownerCalls    int
	decisionCalls int
	executionID   string
	eventID       string
	decision      domainincident.Decision
	actor         string
	reason        string
	payload       any
}

func (f *callbackWorkflowFake) OwnerResponse(_ context.Context, executionID, eventID string, payload any) (*adgo.Execution, error) {
	f.ownerCalls++
	f.executionID = executionID
	f.eventID = eventID
	f.payload = payload
	return &adgo.Execution{ID: executionID}, nil
}

func (f *callbackWorkflowFake) ResolveOwnerDecision(_ context.Context, executionID string, decision domainincident.Decision, actor, reason string, payload any) (*adgo.Execution, error) {
	f.decisionCalls++
	f.executionID = executionID
	f.decision = decision
	f.actor = actor
	f.reason = reason
	f.payload = payload
	return &adgo.Execution{ID: executionID}, nil
}

func newCallbackIngressFixture(role auth.Role) (*CallbackIngress, *callbackAuthorityFake, *callbackUsersFake, *callbackAuditFake, *callbackWorkflowFake) {
	id := testOperatorID
	if role == auth.RoleAdmin {
		id = testAdminID
	}
	security := &callbackAuthorityFake{claims: callback.Claims{KeyID: "active", Subject: id}}
	users := &callbackUsersFake{users: map[string]auth.User{id: {ID: id, Role: role}}}
	audit := &callbackAuditFake{}
	workflow := &callbackWorkflowFake{}
	return &CallbackIngress{Security: security, Users: users, Audit: audit, Workflow: workflow}, security, users, audit, workflow
}

func TestCallbackOwnerResponseUsesFixedBindingAndCurrentOperatorGrant(t *testing.T) {
	ingress, security, _, audit, workflow := newCallbackIngressFixture(auth.RoleOperator)
	payload := map[string]any{"ack": true, "actor": "body-value"}
	execution, err := ingress.OwnerResponse(context.Background(), "opaque", " incident-42 ", " event-7 ", payload, CallbackMeta{RequestID: "req-1", CorrelationID: "cor-1"})
	if err != nil {
		t.Fatal(err)
	}
	if execution.ID != "incident-42" || workflow.ownerCalls != 1 {
		t.Fatalf("execution=%+v ownerCalls=%d", execution, workflow.ownerCalls)
	}
	if security.seen.NodeID != NodeAwaitAck || security.seen.Action != OwnerResponseEvent || security.seen.ExecutionID != "incident-42" || security.seen.EventID != "event-7" {
		t.Fatalf("unexpected callback binding: %+v", security.seen)
	}
	if workflow.executionID != "incident-42" || workflow.eventID != "event-7" {
		t.Fatalf("workflow target changed: execution=%q event=%q", workflow.executionID, workflow.eventID)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("audit entries=%d want=1", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Actor != testOperatorID || entry.Action != OwnerResponseEvent || entry.Result != "allowed" || entry.RequestID != "req-1" || entry.CorrelationID != "cor-1" {
		t.Fatalf("audit entry=%+v", entry)
	}
	var details map[string]any
	if err := json.Unmarshal(entry.Details, &details); err != nil {
		t.Fatal(err)
	}
	if details["capability"] != string(authz.AcknowledgeIncident) {
		t.Fatalf("audit capability=%v", details["capability"])
	}
	if entry.Actor == "body-value" {
		t.Fatal("body actor replaced signed subject")
	}
}

func TestCallbackHighRiskDecisionRequiresAdminAndUsesSignedSubjectAsActor(t *testing.T) {
	operatorIngress, _, _, operatorAudit, operatorWorkflow := newCallbackIngressFixture(auth.RoleOperator)
	_, err := operatorIngress.ResolveOwnerDecision(context.Background(), "opaque", "incident-1", "decision-1", domainincident.DecisionApprove, "approve after review", nil, CallbackMeta{})
	if !errors.Is(err, ErrCallbackForbidden) {
		t.Fatalf("operator high-risk resolution error=%v", err)
	}
	if operatorWorkflow.decisionCalls != 0 {
		t.Fatal("operator reached high-risk workflow")
	}
	if len(operatorAudit.entries) != 1 || operatorAudit.entries[0].Result != "denied" {
		t.Fatalf("operator audit=%+v", operatorAudit.entries)
	}

	adminIngress, security, _, audit, workflow := newCallbackIngressFixture(auth.RoleAdmin)
	execution, err := adminIngress.ResolveOwnerDecision(context.Background(), "opaque", "incident-1", "decision-2", domainincident.DecisionReject, " unsafe ", nil, CallbackMeta{RequestID: "req-2"})
	if err != nil {
		t.Fatal(err)
	}
	if execution.ID != "incident-1" || workflow.decisionCalls != 1 {
		t.Fatalf("execution=%+v decisionCalls=%d", execution, workflow.decisionCalls)
	}
	if security.seen.NodeID != NodeHumanDecision || security.seen.Action != OwnerDecisionEvent || security.seen.EventID != "decision-2" {
		t.Fatalf("decision binding=%+v", security.seen)
	}
	if workflow.actor != testAdminID {
		t.Fatalf("workflow actor=%q want signed subject %q", workflow.actor, testAdminID)
	}
	if workflow.reason != "unsafe" || workflow.decision != domainincident.DecisionReject {
		t.Fatalf("decision=%q reason=%q", workflow.decision, workflow.reason)
	}
	if len(audit.entries) != 1 || audit.entries[0].Actor != testAdminID || audit.entries[0].Result != "allowed" {
		t.Fatalf("admin audit=%+v", audit.entries)
	}
}

func TestCallbackIngressFailsClosedWhenAuthorizationAuditUnavailable(t *testing.T) {
	ingress, _, _, audit, workflow := newCallbackIngressFixture(auth.RoleAdmin)
	audit.err = errors.New("audit unavailable")
	_, err := ingress.ResolveOwnerDecision(context.Background(), "opaque", "incident-1", "decision-1", domainincident.DecisionApprove, "", nil, CallbackMeta{})
	if err == nil || !strings.Contains(err.Error(), "audit callback authorization") {
		t.Fatalf("audit failure error=%v", err)
	}
	if workflow.decisionCalls != 0 {
		t.Fatal("workflow executed without authorization audit")
	}
}
