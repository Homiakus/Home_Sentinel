package fake

import (
	"context"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

func TestNotifierDeduplicatesIdempotencyKey(t *testing.T) {
	n := NewNotifier()
	op := gateway.Operation{ExecutionID: "incident-1", IdempotencyKey: "incident-1:notify"}
	msg := gateway.Notification{Channel: "push", Title: "test", Body: "test"}
	first, err := n.Notify(context.Background(), op, msg)
	if err != nil {
		t.Fatalf("first notify: %v", err)
	}
	second, err := n.Notify(context.Background(), op, msg)
	if err != nil {
		t.Fatalf("second notify: %v", err)
	}
	if first.State != gateway.EffectApplied || second.State != gateway.EffectAlreadyApplied {
		t.Fatalf("unexpected effect states: %s / %s", first.State, second.State)
	}
	if n.Applied != 1 {
		t.Fatalf("semantic effect applied %d times", n.Applied)
	}
}
