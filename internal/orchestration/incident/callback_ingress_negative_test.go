package incident

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

func TestCallbackIngressRejectsInvalidOrDisabledSubjects(t *testing.T) {
	for _, tc := range []struct {
		name      string
		subject   string
		user      auth.User
		wantError error
	}{
		{name: "system principal", subject: "system", wantError: ErrCallbackSubjectInvalid},
		{name: "malformed user id", subject: "usr_short", wantError: ErrCallbackSubjectInvalid},
		{name: "disabled admin", subject: testAdminID, user: auth.User{ID: testAdminID, Role: auth.RoleAdmin, Disabled: true}, wantError: ErrCallbackForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			security := &callbackAuthorityFake{claims: callback.Claims{KeyID: "active", Subject: tc.subject}}
			users := &callbackUsersFake{users: map[string]auth.User{}}
			if tc.user.ID != "" {
				users.users[tc.user.ID] = tc.user
			}
			audit := &callbackAuditFake{}
			workflow := &callbackWorkflowFake{}
			ingress := &CallbackIngress{Security: security, Users: users, Audit: audit, Workflow: workflow}
			_, err := ingress.ResolveOwnerDecision(context.Background(), "opaque", "incident-1", "decision-1", domainincident.DecisionReject, "", nil, CallbackMeta{})
			if !errors.Is(err, tc.wantError) {
				t.Fatalf("error=%v want=%v", err, tc.wantError)
			}
			if workflow.decisionCalls != 0 {
				t.Fatal("unauthorized subject reached workflow")
			}
			if len(audit.entries) != 1 || audit.entries[0].Result != "denied" {
				t.Fatalf("denial audit=%+v", audit.entries)
			}
		})
	}
}

func TestCallbackIngressAuditsVerificationFailureBeforeUserLookup(t *testing.T) {
	security := &callbackAuthorityFake{err: callback.ErrBindingMismatch}
	users := &callbackUsersFake{users: map[string]auth.User{}}
	audit := &callbackAuditFake{}
	workflow := &callbackWorkflowFake{}
	ingress := &CallbackIngress{Security: security, Users: users, Audit: audit, Workflow: workflow}
	_, err := ingress.OwnerResponse(context.Background(), "opaque", "incident-1", "event-1", nil, CallbackMeta{})
	if !errors.Is(err, callback.ErrBindingMismatch) {
		t.Fatalf("error=%v", err)
	}
	if users.calls != 0 || workflow.ownerCalls != 0 {
		t.Fatalf("users=%d workflow=%d after verification failure", users.calls, workflow.ownerCalls)
	}
	if len(audit.entries) != 1 || audit.entries[0].Actor != "external-callback" || audit.entries[0].Result != "denied" {
		t.Fatalf("audit=%+v", audit.entries)
	}
	if !strings.Contains(string(audit.entries[0].Details), "binding_mismatch") {
		t.Fatalf("denial reason missing: %s", audit.entries[0].Details)
	}
}

func TestCallbackIngressRejectsMissingCurrentUser(t *testing.T) {
	security := &callbackAuthorityFake{claims: callback.Claims{KeyID: "active", Subject: testAdminID}}
	users := &callbackUsersFake{users: map[string]auth.User{}}
	audit := &callbackAuditFake{}
	workflow := &callbackWorkflowFake{}
	ingress := &CallbackIngress{Security: security, Users: users, Audit: audit, Workflow: workflow}
	_, err := ingress.ResolveOwnerDecision(context.Background(), "opaque", "incident-1", "decision-1", domainincident.DecisionApprove, "", nil, CallbackMeta{})
	if !errors.Is(err, ErrCallbackForbidden) {
		t.Fatalf("missing user error=%v", err)
	}
	if workflow.decisionCalls != 0 {
		t.Fatal("missing user reached workflow")
	}
}

func TestCallbackIngressRequiresAllDependencies(t *testing.T) {
	var ingress *CallbackIngress
	_, err := ingress.OwnerResponse(context.Background(), "opaque", "incident", "event", nil, CallbackMeta{})
	if !errors.Is(err, ErrCallbackIngressUnavailable) {
		t.Fatalf("nil ingress error=%v", err)
	}
}
