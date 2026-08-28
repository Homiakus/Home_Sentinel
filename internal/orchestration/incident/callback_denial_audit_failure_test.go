package incident

import (
	"errors"
	"strings"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

func TestCallbackVerificationDenialFailsClosedWhenAuditUnavailable(t *testing.T) {
	ingress, security, _, audit, workflow := newCallbackIngressFixture(auth.RoleOperator)
	security.err = callback.ErrInvalidToken
	audit.err = errors.New("audit sink unavailable")

	_, err := ingress.OwnerResponse(t.Context(), "opaque", "execution-1", "event-1", nil, CallbackMeta{})
	if !errors.Is(err, callback.ErrInvalidToken) {
		t.Fatalf("error=%v want invalid token", err)
	}
	if !strings.Contains(err.Error(), "audit callback denial") {
		t.Fatalf("error=%v missing denial audit failure", err)
	}
	if workflow.ownerCalls != 0 {
		t.Fatalf("workflow owner calls=%d want=0", workflow.ownerCalls)
	}
}

func TestCallbackPrincipalDenialFailsClosedWhenAuditUnavailable(t *testing.T) {
	ingress, _, _, audit, workflow := newCallbackIngressFixture(auth.RoleOperator)
	audit.err = errors.New("audit sink unavailable")

	_, err := ingress.ResolveOwnerDecision(
		t.Context(),
		"opaque",
		"execution-1",
		"event-1",
		domainincident.DecisionApprove,
		"reviewed",
		nil,
		CallbackMeta{},
	)
	if !errors.Is(err, ErrCallbackForbidden) {
		t.Fatalf("error=%v want forbidden", err)
	}
	if !strings.Contains(err.Error(), "audit callback denial") {
		t.Fatalf("error=%v missing denial audit failure", err)
	}
	if workflow.decisionCalls != 0 {
		t.Fatalf("workflow decision calls=%d want=0", workflow.decisionCalls)
	}
}
