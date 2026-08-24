package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

type DoorAmbiguity int

const (
	DoorNoAmbiguity DoorAmbiguity = iota
	DoorAmbiguousApplied
	DoorAmbiguousNotApplied
)

type DoorController struct {
	mu        sync.Mutex
	states    map[string]gateway.LockState
	seen      map[string]gateway.EffectResult
	ambiguity DoorAmbiguity
	Calls     int
	Applied   int
}

func NewDoorController(initial map[string]gateway.LockState) *DoorController {
	states := make(map[string]gateway.LockState, len(initial))
	for k, v := range initial {
		states[k] = v
	}
	return &DoorController{states: states, seen: map[string]gateway.EffectResult{}}
}

func (d *DoorController) SetNextAmbiguity(mode DoorAmbiguity) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ambiguity = mode
}

func (d *DoorController) SetObservedState(doorID string, state gateway.LockState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.states[doorID] = state
}

func (d *DoorController) LockState(_ context.Context, doorID string) (gateway.LockState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[doorID]
	if !ok {
		return gateway.LockUnknown, fmt.Errorf("fake door: unknown door %q", doorID)
	}
	return state, nil
}

func (d *DoorController) SetLockState(_ context.Context, op gateway.Operation, doorID string, desired gateway.LockState) (gateway.EffectResult, error) {
	if err := op.Validate(); err != nil {
		return gateway.EffectResult{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Calls++
	if result, ok := d.seen[op.IdempotencyKey]; ok {
		return gateway.EffectResult{State: gateway.EffectAlreadyApplied, ProviderID: result.ProviderID}, nil
	}
	if d.states[doorID] == desired {
		result := gateway.EffectResult{State: gateway.EffectAlreadyApplied, ProviderID: op.IdempotencyKey}
		d.seen[op.IdempotencyKey] = result
		return result, nil
	}
	mode := d.ambiguity
	d.ambiguity = DoorNoAmbiguity
	switch mode {
	case DoorAmbiguousApplied:
		d.states[doorID] = desired
		d.Applied++
		return gateway.EffectResult{State: gateway.EffectAmbiguous, ProviderID: op.IdempotencyKey}, nil
	case DoorAmbiguousNotApplied:
		return gateway.EffectResult{State: gateway.EffectAmbiguous, ProviderID: op.IdempotencyKey}, nil
	default:
		d.states[doorID] = desired
		d.Applied++
		result := gateway.EffectResult{State: gateway.EffectApplied, ProviderID: op.IdempotencyKey}
		d.seen[op.IdempotencyKey] = result
		return result, nil
	}
}
