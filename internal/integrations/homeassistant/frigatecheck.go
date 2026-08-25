package homeassistant

import (
	"context"
	"strings"
)

type FrigateCheck struct {
	ComponentLoaded  bool     `json:"component_loaded"`
	ExpectedEntities []string `json:"expected_entities,omitempty"`
	MissingEntities  []string `json:"missing_entities,omitempty"`
	Recommendation   string   `json:"recommendation,omitempty"`
}

func VerifyFrigateIntegration(ctx context.Context, rest *RESTClient, expectedEntities []string) (FrigateCheck, error) {
	cfg, err := rest.Config(ctx)
	if err != nil {
		return FrigateCheck{}, err
	}
	out := FrigateCheck{ComponentLoaded: HasComponent(cfg, "frigate"), ExpectedEntities: append([]string(nil), expectedEntities...)}
	if !out.ComponentLoaded {
		out.Recommendation = "Install/configure the documented Frigate integration in Home Assistant, then run verification again. Sentinel will not modify Home Assistant .storage or emulate the config flow."
		return out, nil
	}
	if len(expectedEntities) == 0 {
		return out, nil
	}
	states, err := rest.States(ctx)
	if err != nil {
		return FrigateCheck{}, err
	}
	seen := map[string]struct{}{}
	for _, s := range states {
		seen[s.EntityID] = struct{}{}
	}
	for _, id := range expectedEntities {
		if !entityIDPattern.MatchString(id) || !strings.HasPrefix(id, "camera.") {
			out.MissingEntities = append(out.MissingEntities, id)
			continue
		}
		if _, ok := seen[id]; !ok {
			out.MissingEntities = append(out.MissingEntities, id)
		}
	}
	if len(out.MissingEntities) > 0 {
		out.Recommendation = "Frigate is loaded, but expected camera entities are missing. Verify the Frigate integration camera configuration in Home Assistant."
	}
	return out, nil
}
