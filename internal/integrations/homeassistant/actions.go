package homeassistant

import (
	"context"
	"errors"
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

type ActionBinding struct {
	Name    string         `json:"name"`
	Action  Action         `json:"action"`
	Payload map[string]any `json:"payload,omitempty"`
}

type ActionResult struct {
	Binding       string    `json:"binding"`
	Actor         string    `json:"actor"`
	CorrelationID domain.ID `json:"correlation_id"`
	Changed       []State   `json:"changed,omitempty"`
}

type ActionBridge struct {
	REST     *RESTClient
	bindings map[string]ActionBinding
}

func NewActionBridge(rest *RESTClient, bindings []ActionBinding) (*ActionBridge, error) {
	if rest == nil {
		return nil, errors.New("Home Assistant REST client required")
	}
	m := make(map[string]ActionBinding, len(bindings))
	for _, b := range bindings {
		if b.Name == "" {
			return nil, errors.New("Home Assistant action binding name required")
		}
		if _, exists := m[b.Name]; exists {
			return nil, fmt.Errorf("duplicate Home Assistant action binding %q", b.Name)
		}
		if err := validateAction(b.Action); err != nil {
			return nil, err
		}
		b.Payload = cloneMap(b.Payload)
		m[b.Name] = b
	}
	return &ActionBridge{REST: rest, bindings: m}, nil
}

func (b *ActionBridge) Execute(ctx context.Context, name, actor string, correlationID domain.ID) (ActionResult, error) {
	if b == nil || b.REST == nil {
		return ActionResult{}, errors.New("Home Assistant action bridge unavailable")
	}
	binding, ok := b.bindings[name]
	if !ok {
		return ActionResult{}, fmt.Errorf("Home Assistant action binding %q is not authorized", name)
	}
	if actor == "" {
		return ActionResult{}, errors.New("actor required")
	}
	if !correlationID.ValidFor("cor") {
		return ActionResult{}, errors.New("valid correlation id required")
	}
	changed, err := b.REST.CallAction(ctx, binding.Action, cloneMap(binding.Payload))
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Binding: name, Actor: actor, CorrelationID: correlationID, Changed: changed}, nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
