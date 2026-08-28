package callback

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBindingInvalid  = errors.New("callback: expected binding is incomplete")
	ErrBindingMismatch = errors.New("callback: token binding mismatch")
)

// Binding is the exact semantic callback target allowed by an ingress token.
// All fields are mandatory at the Stage-17 acceptance boundary.
type Binding struct {
	ExecutionID string
	NodeID      string
	EventID     string
	Action      string
}

func (b Binding) Validate() error {
	if strings.TrimSpace(b.ExecutionID) == "" || strings.TrimSpace(b.NodeID) == "" || strings.TrimSpace(b.EventID) == "" || strings.TrimSpace(b.Action) == "" {
		return ErrBindingInvalid
	}
	return nil
}

// Matches is deliberately exact: no wildcard or prefix semantics are allowed
// at the external callback boundary.
func (b Binding) Matches(claims Claims) bool {
	return claims.ExecutionID == b.ExecutionID &&
		claims.NodeID == b.NodeID &&
		claims.EventID == b.EventID &&
		claims.Action == b.Action
}

// Acceptor combines cryptographic verification, exact semantic binding and
// bounded replay admission into one fail-closed operation. Replay state is
// consumed only after every cryptographic and binding check succeeds, so an
// attacker cannot burn a valid token merely by presenting it to a wrong route.
type Acceptor struct {
	keys   *Keyring
	replay *ReplayGuard
	now    func() time.Time
}

func NewAcceptor(keys *Keyring, replay *ReplayGuard) (*Acceptor, error) {
	if keys == nil {
		return nil, ErrInvalidKey
	}
	if replay == nil {
		return nil, errors.New("callback: replay guard is required")
	}
	return &Acceptor{keys: keys, replay: replay, now: time.Now}, nil
}

func (a *Acceptor) Accept(token string, expected Binding) (Claims, error) {
	if a == nil || a.keys == nil || a.replay == nil {
		return Claims{}, errors.New("callback: acceptor is not configured")
	}
	if err := expected.Validate(); err != nil {
		return Claims{}, err
	}
	claims, err := a.keys.Verify(token)
	if err != nil {
		return Claims{}, err
	}
	if !expected.Matches(claims) {
		return Claims{}, ErrBindingMismatch
	}
	now := time.Now().UTC()
	if a.now != nil {
		now = a.now().UTC()
	}
	if err := a.replay.Consume(claims, now); err != nil {
		// Preserve already-verified, exactly-bound claims on replay. The strict
		// replay error remains intact, while higher layers can reconcile the
		// duplicate against durable domain state without re-verifying or parsing
		// untrusted token material a second time.
		return claims, err
	}
	return claims, nil
}
