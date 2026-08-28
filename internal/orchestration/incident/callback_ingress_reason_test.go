package incident

import (
	"errors"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

func TestCallbackReasonClassifiesSafeAuditCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "binding invalid", err: callback.ErrBindingInvalid, want: "binding_invalid"},
		{name: "binding mismatch", err: callback.ErrBindingMismatch, want: "binding_mismatch"},
		{name: "replay", err: callback.ErrReplay, want: "replay"},
		{name: "expired", err: callback.ErrExpired, want: "expired"},
		{name: "invalid token", err: callback.ErrInvalidToken, want: "invalid_token"},
		{name: "wrapped replay", err: errors.Join(errors.New("outer"), callback.ErrReplay), want: "replay"},
		{name: "unknown", err: errors.New("other"), want: "verification_failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := callbackReason(tc.err); got != tc.want {
				t.Fatalf("callbackReason(%v)=%q want=%q", tc.err, got, tc.want)
			}
		})
	}
}

func TestCallbackIngressRejectsEachMissingDependency(t *testing.T) {
	valid, security, users, audit, workflow := newCallbackIngressFixture("admin")
	_ = valid
	cases := []struct {
		name    string
		ingress *CallbackIngress
	}{
		{name: "nil receiver", ingress: nil},
		{name: "security", ingress: &CallbackIngress{Users: users, Audit: audit, Workflow: workflow}},
		{name: "users", ingress: &CallbackIngress{Security: security, Audit: audit, Workflow: workflow}},
		{name: "audit", ingress: &CallbackIngress{Security: security, Users: users, Workflow: workflow}},
		{name: "workflow", ingress: &CallbackIngress{Security: security, Users: users, Audit: audit}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.ingress.OwnerResponse(t.Context(), "opaque", "execution", "event", nil, CallbackMeta{})
			if !errors.Is(err, ErrCallbackIngressUnavailable) {
				t.Fatalf("error=%v want ErrCallbackIngressUnavailable", err)
			}
		})
	}
}
