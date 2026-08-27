package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

type CameraRecoveryController struct {
	mu             sync.Mutex
	network        map[string]bool
	stream         map[string]bool
	seen           map[string]gateway.EffectResult
	ReconnectCalls int
}

func NewCameraRecoveryController(network, stream map[string]bool) *CameraRecoveryController {
	n := make(map[string]bool, len(network))
	s := make(map[string]bool, len(stream))
	for id, value := range network {
		n[id] = value
	}
	for id, value := range stream {
		s[id] = value
	}
	return &CameraRecoveryController{network: n, stream: s, seen: map[string]gateway.EffectResult{}}
}

func (c *CameraRecoveryController) SetNetwork(cameraID string, value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.network[cameraID] = value
}

func (c *CameraRecoveryController) SetStream(cameraID string, value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stream[cameraID] = value
}

func (c *CameraRecoveryController) ProbeNetwork(_ context.Context, cameraID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.network[cameraID]
	if !ok {
		return false, fmt.Errorf("fake camera recovery: unknown camera %q", cameraID)
	}
	return value, nil
}

func (c *CameraRecoveryController) ProbeStream(_ context.Context, cameraID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.stream[cameraID]
	if !ok {
		return false, fmt.Errorf("fake camera recovery: unknown camera %q", cameraID)
	}
	return value, nil
}

func (c *CameraRecoveryController) Reconnect(_ context.Context, op gateway.Operation, cameraID string) (gateway.EffectResult, error) {
	if err := op.Validate(); err != nil {
		return gateway.EffectResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if result, ok := c.seen[op.IdempotencyKey]; ok {
		return gateway.EffectResult{State: gateway.EffectAlreadyApplied, ProviderID: result.ProviderID}, nil
	}
	if _, ok := c.network[cameraID]; !ok {
		return gateway.EffectResult{}, fmt.Errorf("fake camera recovery: unknown camera %q", cameraID)
	}
	c.ReconnectCalls++
	if c.network[cameraID] {
		c.stream[cameraID] = true
	}
	result := gateway.EffectResult{State: gateway.EffectApplied, ProviderID: op.IdempotencyKey}
	c.seen[op.IdempotencyKey] = result
	return result, nil
}
