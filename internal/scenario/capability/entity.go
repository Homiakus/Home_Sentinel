package capability

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type EntityDescriptor struct {
	ID            string       `json:"id"`
	Kind          string       `json:"kind"`
	ProviderID    string       `json:"providerId"`
	IntegrationID string       `json:"integrationId"`
	Title         string       `json:"title"`
	Description   string       `json:"description,omitempty"`
	Area          string       `json:"area,omitempty"`
	Capabilities  []Key        `json:"capabilities"`
	Visibility    Visibility   `json:"visibility"`
	Available     bool         `json:"available"`
	Health        HealthStatus `json:"health"`
}

func (e EntityDescriptor) Validate() error {
	if err := validateID("entity id", e.ID); err != nil {
		return err
	}
	if err := validateID("entity kind", e.Kind); err != nil {
		return err
	}
	if err := validateID("provider id", e.ProviderID); err != nil {
		return err
	}
	if err := validateID("integration id", e.IntegrationID); err != nil {
		return err
	}
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("capability: entity title is required")
	}
	if !e.Visibility.valid() {
		return fmt.Errorf("capability: invalid entity visibility %q", e.Visibility)
	}
	if !validHealth(e.Health) {
		return fmt.Errorf("capability: invalid entity health %q", e.Health)
	}
	if len(e.Capabilities) == 0 {
		return fmt.Errorf("capability: entity %q has no capabilities", e.ID)
	}
	seen := map[string]struct{}{}
	for _, key := range e.Capabilities {
		if err := validateID("capability id", key.ID); err != nil {
			return err
		}
		if _, err := ParseSemVer(key.Version); err != nil {
			return err
		}
		if _, exists := seen[key.String()]; exists {
			return fmt.Errorf("capability: entity %q contains duplicate capability %q", e.ID, key.String())
		}
		seen[key.String()] = struct{}{}
	}
	return nil
}

func normalizeEntity(source EntityDescriptor) (EntityDescriptor, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return EntityDescriptor{}, err
	}
	var entity EntityDescriptor
	if err := json.Unmarshal(raw, &entity); err != nil {
		return EntityDescriptor{}, err
	}
	entity.ID = strings.TrimSpace(entity.ID)
	entity.Kind = strings.TrimSpace(entity.Kind)
	entity.ProviderID = strings.TrimSpace(entity.ProviderID)
	entity.IntegrationID = strings.TrimSpace(entity.IntegrationID)
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Description = strings.TrimSpace(entity.Description)
	entity.Area = strings.TrimSpace(entity.Area)
	for i := range entity.Capabilities {
		entity.Capabilities[i].ID = strings.TrimSpace(entity.Capabilities[i].ID)
		entity.Capabilities[i].Version = strings.TrimSpace(entity.Capabilities[i].Version)
	}
	sort.Slice(entity.Capabilities, func(i, j int) bool {
		return entity.Capabilities[i].String() < entity.Capabilities[j].String()
	})
	if err := entity.Validate(); err != nil {
		return EntityDescriptor{}, err
	}
	return entity, nil
}
