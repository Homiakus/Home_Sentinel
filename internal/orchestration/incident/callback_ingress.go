package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/authz"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
	"github.com/Homiakus/axiom/adgo"
)

var (
	ErrCallbackIngressUnavailable = errors.New("incident: callback ingress unavailable")
	ErrCallbackSubjectInvalid     = errors.New("incident: callback subject must be a human user id")
	ErrCallbackForbidden          = errors.New("incident: callback principal is not authorized")
)

type CallbackAuthority interface {
	Accept(token string, expected callback.Binding) (callback.Claims, error)
}

type CallbackUserStore interface {
	GetByID(context.Context, string) (auth.User, error)
}

type CallbackAuditStore interface {
	Append(context.Context, repository.AuditEntry) (repository.AuditEntry, error)
}

type CallbackWorkflow interface {
	OwnerResponse(context.Context, string, string, any) (*adgo.Execution, error)
	ResolveOwnerDecision(context.Context, string, domainincident.Decision, string, string, any) (*adgo.Execution, error)
}

type CallbackIngress struct {
	Security CallbackAuthority
	Users    CallbackUserStore
	Audit    CallbackAuditStore
	Workflow CallbackWorkflow
}

type CallbackMeta struct {
	RequestID     string
	CorrelationID string
}

func (s *CallbackIngress) OwnerResponse(
	ctx context.Context,
	token string,
	executionID string,
	eventID string,
	payload any,
	meta CallbackMeta,
) (*adgo.Execution, error) {
	expected := callback.Binding{
		ExecutionID: strings.TrimSpace(executionID),
		NodeID:      NodeAwaitAck,
		EventID:     strings.TrimSpace(eventID),
		Action:      OwnerResponseEvent,
	}
	user, _, err := s.authorize(ctx, token, expected, authz.AcknowledgeIncident, meta)
	if err != nil {
		return nil, err
	}
	return s.Workflow.OwnerResponse(ctx, expected.ExecutionID, expected.EventID, payload)
}

func (s *CallbackIngress) ResolveOwnerDecision(
	ctx context.Context,
	token string,
	executionID string,
	eventID string,
	decision domainincident.Decision,
	reason string,
	payload any,
	meta CallbackMeta,
) (*adgo.Execution, error) {
	expected := callback.Binding{
		ExecutionID: strings.TrimSpace(executionID),
		NodeID:      NodeHumanDecision,
		EventID:     strings.TrimSpace(eventID),
		Action:      OwnerDecisionEvent,
	}
	user, _, err := s.authorize(ctx, token, expected, authz.ResolveIncident, meta)
	if err != nil {
		return nil, err
	}
	return s.Workflow.ResolveOwnerDecision(ctx, expected.ExecutionID, decision, user.ID, strings.TrimSpace(reason), payload)
}

func (s *CallbackIngress) authorize(
	ctx context.Context,
	token string,
	expected callback.Binding,
	capability authz.Capability,
	meta CallbackMeta,
) (auth.User, callback.Claims, error) {
	if s == nil || s.Security == nil || s.Users == nil || s.Audit == nil || s.Workflow == nil {
		return auth.User{}, callback.Claims{}, ErrCallbackIngressUnavailable
	}
	claims, err := s.Security.Accept(token, expected)
	if err != nil {
		auditErr := s.auditDecision(ctx, "external-callback", expected, "denied", callbackReason(err), "", capability, meta)
		if auditErr != nil {
			return auth.User{}, callback.Claims{}, errors.Join(err, fmt.Errorf("audit callback denial: %w", auditErr))
		}
		return auth.User{}, callback.Claims{}, err
	}
	if !domain.ID(claims.Subject).ValidFor("usr") {
		err = ErrCallbackSubjectInvalid
		auditErr := s.auditDecision(ctx, "external-callback", expected, "denied", "invalid_subject", claims.KeyID, capability, meta)
		if auditErr != nil {
			return auth.User{}, callback.Claims{}, errors.Join(err, fmt.Errorf("audit callback denial: %w", auditErr))
		}
		return auth.User{}, callback.Claims{}, err
	}
	user, userErr := s.Users.GetByID(ctx, claims.Subject)
	if userErr != nil || user.Disabled || !authz.Allowed(user.Role, capability) {
		err = ErrCallbackForbidden
		auditErr := s.auditDecision(ctx, claims.Subject, expected, "denied", "principal_not_authorized", claims.KeyID, capability, meta)
		if auditErr != nil {
			return auth.User{}, callback.Claims{}, errors.Join(err, fmt.Errorf("audit callback denial: %w", auditErr))
		}
		return auth.User{}, callback.Claims{}, err
	}
	if err := s.auditDecision(ctx, user.ID, expected, "allowed", "", claims.KeyID, capability, meta); err != nil {
		return auth.User{}, callback.Claims{}, fmt.Errorf("audit callback authorization: %w", err)
	}
	return user, claims, nil
}

func (s *CallbackIngress) auditDecision(
	ctx context.Context,
	actor string,
	expected callback.Binding,
	result string,
	reasonCode string,
	keyID string,
	capability authz.Capability,
	meta CallbackMeta,
) error {
	details, err := json.Marshal(struct {
		NodeID     string `json:"node_id"`
		EventID    string `json:"event_id"`
		KeyID      string `json:"key_id,omitempty"`
		Capability string `json:"capability"`
		ReasonCode string `json:"reason_code,omitempty"`
	}{
		NodeID:     expected.NodeID,
		EventID:    expected.EventID,
		KeyID:      keyID,
		Capability: string(capability),
		ReasonCode: reasonCode,
	})
	if err != nil {
		return err
	}
	_, err = s.Audit.Append(ctx, repository.AuditEntry{
		Actor:         actor,
		Source:        "callback",
		Action:        expected.Action,
		Target:        expected.ExecutionID + "/" + expected.NodeID,
		Result:        result,
		RequestID:     strings.TrimSpace(meta.RequestID),
		CorrelationID: strings.TrimSpace(meta.CorrelationID),
		Details:       details,
	})
	return err
}

func callbackReason(err error) string {
	switch {
	case errors.Is(err, callback.ErrBindingInvalid):
		return "binding_invalid"
	case errors.Is(err, callback.ErrBindingMismatch):
		return "binding_mismatch"
	case errors.Is(err, callback.ErrReplay):
		return "replay"
	case errors.Is(err, callback.ErrExpired):
		return "expired"
	case errors.Is(err, callback.ErrInvalidToken):
		return "invalid_token"
	default:
		return "verification_failed"
	}
}
