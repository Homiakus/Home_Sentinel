package gateway

import (
	"errors"
	"testing"
)

func TestOperationRequiresExecutionAndIdempotencyIdentity(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		want error
	}{
		{name: "valid", op: Operation{ExecutionID: "exec-1", IdempotencyKey: "effect-1"}},
		{name: "missing execution", op: Operation{IdempotencyKey: "effect-1"}, want: ErrMissingExecutionID},
		{name: "missing idempotency", op: Operation{ExecutionID: "exec-1"}, want: ErrMissingIdempotencyKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if tt.want == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}
