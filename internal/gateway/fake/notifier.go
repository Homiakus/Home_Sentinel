package fake

import (
	"context"
	"sync"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

// Notifier is an in-memory idempotent gateway used by workflow contract tests
// and local development. Repeating the same idempotency key never represents a
// second semantic notification.
type Notifier struct {
	mu      sync.Mutex
	seen    map[string]gateway.Notification
	Calls   int
	Applied int
}

func NewNotifier() *Notifier {
	return &Notifier{seen: map[string]gateway.Notification{}}
}

func (n *Notifier) Notify(_ context.Context, op gateway.Operation, notification gateway.Notification) (gateway.EffectResult, error) {
	if err := op.Validate(); err != nil {
		return gateway.EffectResult{}, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Calls++
	if _, ok := n.seen[op.IdempotencyKey]; ok {
		return gateway.EffectResult{State: gateway.EffectAlreadyApplied, ProviderID: op.IdempotencyKey}, nil
	}
	n.seen[op.IdempotencyKey] = notification
	n.Applied++
	return gateway.EffectResult{State: gateway.EffectApplied, ProviderID: op.IdempotencyKey}, nil
}
