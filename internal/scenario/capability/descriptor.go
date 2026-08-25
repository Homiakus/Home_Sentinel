package capability

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type Kind string

const (
	KindTrigger Kind = "trigger"
	KindAction  Kind = "action"
	KindState   Kind = "state"
)

type Category string

type IdempotencySemantics string

const (
	IdempotencyNone      IdempotencySemantics = "none"
	IdempotencySupported IdempotencySemantics = "supported"
	IdempotencyRequired  IdempotencySemantics = "required"
)

type VerificationSupport string

const (
	VerificationNone        VerificationSupport = "none"
	VerificationProviderAck VerificationSupport = "provider_ack"
	VerificationReadback    VerificationSupport = "readback"
)

type CompensationClass string

const (
	CompensationNone      CompensationClass = "none"
	CompensationAutomatic CompensationClass = "automatic"
	CompensationManual    CompensationClass = "manual"
	CompensationReconcile CompensationClass = "reconcile"
)

type HealthStatus string

const (
	HealthUnknown  HealthStatus = "unknown"
	HealthHealthy  HealthStatus = "healthy"
	HealthDegraded HealthStatus = "degraded"
	HealthOffline  HealthStatus = "offline"
)

type Key struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

func (k Key) String() string { return k.ID + "@" + k.Version }

type Descriptor struct {
	ID             string               `json:"id"`
	Version        string               `json:"version"`
	ProviderID     string               `json:"providerId"`
	IntegrationID  string               `json:"integrationId"`
	Kind           Kind                 `json:"kind"`
	Category       Category             `json:"category"`
	Title          string               `json:"title"`
	Description    string               `json:"description,omitempty"`
	EntityKinds    []string             `json:"entityKinds,omitempty"`
	Input          Schema               `json:"input,omitempty"`
	Output         Schema               `json:"output,omitempty"`
	State          Schema               `json:"state,omitempty"`
	Risk           model.RiskLevel      `json:"risk"`
	Permission     Permission           `json:"permission"`
	Visibility     Visibility           `json:"visibility"`
	Reversible     bool                 `json:"reversible,omitempty"`
	ExternalEffect bool                 `json:"externalEffect,omitempty"`
	Idempotency    IdempotencySemantics `json:"idempotency"`
	Verification   VerificationSupport  `json:"verification"`
	Compensation   CompensationClass    `json:"compensation"`
	UI             UIHints              `json:"ui,omitempty"`
	Available      bool                 `json:"available"`
	Health         HealthStatus         `json:"health"`
}

func NewDescriptor(kind Kind, id, version, providerID, integrationID string, category Category, title string, permission Permission) (Descriptor, error) {
	return NormalizeDescriptor(Descriptor{
		ID:            id,
		Version:       version,
		ProviderID:    providerID,
		IntegrationID: integrationID,
		Kind:          kind,
		Category:      category,
		Title:         title,
		Risk:          model.RiskLow,
		Permission:    permission,
		Visibility:    VisibilityPublic,
		Idempotency:   IdempotencyNone,
		Verification:  VerificationNone,
		Compensation:  CompensationNone,
		Available:     true,
		Health:        HealthHealthy,
	})
}

func (d Descriptor) Key() Key { return Key{ID: d.ID, Version: d.Version} }

func (d Descriptor) Validate() error {
	if err := validateID("capability id", d.ID); err != nil {
		return err
	}
	if _, err := ParseSemVer(d.Version); err != nil {
		return err
	}
	if err := validateID("provider id", d.ProviderID); err != nil {
		return err
	}
	if err := validateID("integration id", d.IntegrationID); err != nil {
		return err
	}
	switch d.Kind {
	case KindTrigger, KindAction, KindState:
	default:
		return fmt.Errorf("capability: invalid kind %q", d.Kind)
	}
	if strings.TrimSpace(string(d.Category)) == "" {
		return fmt.Errorf("capability: category is required")
	}
	if strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("capability: title is required")
	}
	if err := d.Risk.Validate(); err != nil {
		return err
	}
	if err := validateID("permission", string(d.Permission)); err != nil {
		return err
	}
	if !d.Visibility.valid() {
		return fmt.Errorf("capability: invalid visibility %q", d.Visibility)
	}
	if err := d.Input.Validate(); err != nil {
		return err
	}
	if err := d.Output.Validate(); err != nil {
		return err
	}
	if err := d.State.Validate(); err != nil {
		return err
	}
	if !validIdempotency(d.Idempotency) {
		return fmt.Errorf("capability: invalid idempotency semantics %q", d.Idempotency)
	}
	if !validVerification(d.Verification) {
		return fmt.Errorf("capability: invalid verification support %q", d.Verification)
	}
	if !validCompensation(d.Compensation) {
		return fmt.Errorf("capability: invalid compensation class %q", d.Compensation)
	}
	if !validHealth(d.Health) {
		return fmt.Errorf("capability: invalid health %q", d.Health)
	}
	if d.ExternalEffect && d.Kind != KindAction {
		return fmt.Errorf("capability: only actions may declare external effects")
	}
	if d.ExternalEffect && d.Idempotency == IdempotencyNone {
		return fmt.Errorf("capability: external effect %q must declare idempotency semantics", d.ID)
	}
	for _, kind := range d.EntityKinds {
		if err := validateID("entity kind", kind); err != nil {
			return err
		}
	}
	return nil
}

func NormalizeDescriptor(source Descriptor) (Descriptor, error) {
	descriptor, err := cloneDescriptor(source)
	if err != nil {
		return Descriptor{}, err
	}
	descriptor.ID = strings.TrimSpace(descriptor.ID)
	descriptor.Version = strings.TrimSpace(descriptor.Version)
	descriptor.ProviderID = strings.TrimSpace(descriptor.ProviderID)
	descriptor.IntegrationID = strings.TrimSpace(descriptor.IntegrationID)
	descriptor.Category = Category(strings.TrimSpace(string(descriptor.Category)))
	descriptor.Title = strings.TrimSpace(descriptor.Title)
	descriptor.Description = strings.TrimSpace(descriptor.Description)
	descriptor.EntityKinds = normalizeStrings(descriptor.EntityKinds)
	descriptor.Input = normalizeSchema(descriptor.Input)
	descriptor.Output = normalizeSchema(descriptor.Output)
	descriptor.State = normalizeSchema(descriptor.State)
	descriptor.UI = normalizeUIHints(descriptor.UI)
	if err := descriptor.Validate(); err != nil {
		return Descriptor{}, err
	}
	return descriptor, nil
}

func cloneDescriptor(source Descriptor) (Descriptor, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return Descriptor{}, err
	}
	var out Descriptor
	if err := json.Unmarshal(raw, &out); err != nil {
		return Descriptor{}, err
	}
	return out, nil
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func validateID(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 160 {
		return fmt.Errorf("capability: %s is invalid", label)
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '-', '_', '.', ':', '/':
			continue
		default:
			return fmt.Errorf("capability: %s %q contains unsupported characters", label, value)
		}
	}
	return nil
}

func validIdempotency(value IdempotencySemantics) bool {
	return value == IdempotencyNone || value == IdempotencySupported || value == IdempotencyRequired
}

func validVerification(value VerificationSupport) bool {
	return value == VerificationNone || value == VerificationProviderAck || value == VerificationReadback
}

func validCompensation(value CompensationClass) bool {
	return value == CompensationNone || value == CompensationAutomatic || value == CompensationManual || value == CompensationReconcile
}

func validHealth(value HealthStatus) bool {
	return value == HealthUnknown || value == HealthHealthy || value == HealthDegraded || value == HealthOffline
}
