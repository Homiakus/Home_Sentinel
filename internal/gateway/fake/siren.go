package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

type SirenController struct {
	mu      sync.Mutex
	states  map[string]bool
	seen    map[string]gateway.EffectResult
	Calls   int
	Applied int
}

func NewSirenController(initial map[string]bool) *SirenController {
	states := make(map[string]bool, len(initial))
	for id, enabled := range initial {
		states[id] = enabled
	}
	return &SirenController{states: states, seen: map[string]gateway.EffectResult{}}
}

func (s *SirenController) Enabled(_ context.Context, sirenID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.states[sirenID]
	if !ok {
		return false, fmt.Errorf("fake siren: unknown siren %q", sirenID)
	}
	return value, nil
}

func (s *SirenController) SetEnabled(_ context.Context, op gateway.Operation, sirenID string, desired bool) (gateway.EffectResult, error) {
	if err := op.Validate(); err != nil {
		return gateway.EffectResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls++
	if result, ok := s.seen[op.IdempotencyKey]; ok {
		return gateway.EffectResult{State: gateway.EffectAlreadyApplied, ProviderID: result.ProviderID}, nil
	}
	current, ok := s.states[sirenID]
	if !ok {
		return gateway.EffectResult{}, fmt.Errorf("fake siren: unknown siren %q", sirenID)
	}
	if current == desired {
		result := gateway.EffectResult{State: gateway.EffectAlreadyApplied, ProviderID: op.IdempotencyKey}
		s.seen[op.IdempotencyKey] = result
		return result, nil
	}
	s.states[sirenID] = desired
	s.Applied++
	result := gateway.EffectResult{State: gateway.EffectApplied, ProviderID: op.IdempotencyKey}
	s.seen[op.IdempotencyKey] = result
	return result, nil
}
