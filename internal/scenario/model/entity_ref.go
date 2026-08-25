package model

import (
	"fmt"
	"strings"
)

type EntityRef struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

func (r EntityRef) Validate() error {
	if err := validateToken("entity id", r.ID); err != nil {
		return err
	}
	if err := validateToken("entity kind", r.Kind); err != nil {
		return err
	}
	return nil
}

type CapabilityRef struct {
	ID      string     `json:"id"`
	Version string     `json:"version"`
	Entity  *EntityRef `json:"entity,omitempty"`
}

func (r CapabilityRef) Validate() error {
	if err := validateToken("capability id", r.ID); err != nil {
		return err
	}
	if strings.TrimSpace(r.Version) == "" {
		return fmt.Errorf("scenario: capability %q version is required", r.ID)
	}
	if len(r.Version) > 64 {
		return fmt.Errorf("scenario: capability %q version is too long", r.ID)
	}
	if r.Entity != nil {
		if err := r.Entity.Validate(); err != nil {
			return fmt.Errorf("scenario: capability %q entity: %w", r.ID, err)
		}
	}
	return nil
}
