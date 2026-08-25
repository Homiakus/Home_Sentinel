package model

import (
	"fmt"
	"strings"
)

type TriggerKind string

const (
	TriggerDeviceEvent TriggerKind = "device_event"
	TriggerStateChange TriggerKind = "state_change"
	TriggerThreshold   TriggerKind = "threshold"
	TriggerSchedule    TriggerKind = "schedule"
	TriggerPresence    TriggerKind = "presence"
	TriggerSystem      TriggerKind = "system"
	TriggerWebhook     TriggerKind = "webhook"
	TriggerManual      TriggerKind = "manual"
	TriggerCompound    TriggerKind = "compound"
)

func (k TriggerKind) valid() bool {
	switch k {
	case TriggerDeviceEvent, TriggerStateChange, TriggerThreshold, TriggerSchedule, TriggerPresence, TriggerSystem, TriggerWebhook, TriggerManual, TriggerCompound:
		return true
	default:
		return false
	}
}

type Trigger struct {
	ID         string           `json:"id"`
	Kind       TriggerKind      `json:"kind"`
	Capability CapabilityRef    `json:"capability"`
	Parameters map[string]Value `json:"parameters,omitempty"`
	Filter     Expr             `json:"filter,omitempty"`
	Temporal   []TemporalSpec   `json:"temporal,omitempty"`
}

func (t Trigger) Validate() error {
	if err := validateToken("trigger id", t.ID); err != nil {
		return err
	}
	if !t.Kind.valid() {
		return fmt.Errorf("scenario: unknown trigger kind %q", t.Kind)
	}
	if err := t.Capability.Validate(); err != nil {
		return fmt.Errorf("scenario: trigger %q: %w", t.ID, err)
	}
	if err := validateValues("trigger parameter", t.Parameters); err != nil {
		return fmt.Errorf("scenario: trigger %q: %w", t.ID, err)
	}
	if !t.Filter.IsZero() {
		if err := t.Filter.Validate(); err != nil {
			return fmt.Errorf("scenario: trigger %q filter: %w", t.ID, err)
		}
	}
	for i := range t.Temporal {
		if err := t.Temporal[i].Validate(); err != nil {
			return fmt.Errorf("scenario: trigger %q temporal[%d]: %w", t.ID, i, err)
		}
	}
	return nil
}

func normalizeTrigger(t Trigger) (Trigger, error) {
	t.ID = strings.TrimSpace(t.ID)
	normalizeCapability(&t.Capability)
	if err := normalizeValues(t.Parameters); err != nil {
		return Trigger{}, err
	}
	filter, err := normalizeExpr(t.Filter)
	if err != nil {
		return Trigger{}, err
	}
	t.Filter = filter
	for i := range t.Temporal {
		normalized, err := NormalizeTemporal(t.Temporal[i])
		if err != nil {
			return Trigger{}, err
		}
		t.Temporal[i] = normalized
	}
	if err := t.Validate(); err != nil {
		return Trigger{}, err
	}
	return t, nil
}
