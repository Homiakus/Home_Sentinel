package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func Normalize(source Scenario) (Scenario, error) {
	scenario, err := deepCopyScenario(source)
	if err != nil {
		return Scenario{}, err
	}
	scenario.ID = ID(strings.TrimSpace(string(scenario.ID)))
	scenario.RevisionID = RevisionID(strings.TrimSpace(string(scenario.RevisionID)))
	scenario.Name = strings.TrimSpace(scenario.Name)
	scenario.Description = strings.TrimSpace(scenario.Description)
	for i := range scenario.Triggers {
		normalized, err := normalizeTrigger(scenario.Triggers[i])
		if err != nil {
			return Scenario{}, fmt.Errorf("scenario: normalize trigger[%d]: %w", i, err)
		}
		scenario.Triggers[i] = normalized
	}
	// Triggers are an OR-set. Their presentation order is not execution semantics.
	sort.SliceStable(scenario.Triggers, func(i, j int) bool { return scenario.Triggers[i].ID < scenario.Triggers[j].ID })
	condition, err := normalizeExpr(scenario.Condition)
	if err != nil {
		return Scenario{}, err
	}
	scenario.Condition = condition
	flow, err := normalizeFlow(scenario.Flow)
	if err != nil {
		return Scenario{}, err
	}
	scenario.Flow = flow
	for i := range scenario.Parameters {
		scenario.Parameters[i].ID = strings.TrimSpace(scenario.Parameters[i].ID)
		typ, err := scenario.Parameters[i].Type.Normalize()
		if err != nil {
			return Scenario{}, fmt.Errorf("scenario: normalize parameter %q type: %w", scenario.Parameters[i].ID, err)
		}
		scenario.Parameters[i].Type = typ
		if scenario.Parameters[i].Default != nil {
			canonical, err := scenario.Parameters[i].Default.canonical()
			if err != nil {
				return Scenario{}, err
			}
			scenario.Parameters[i].Default = &canonical
		}
	}
	sort.SliceStable(scenario.Parameters, func(i, j int) bool { return scenario.Parameters[i].ID < scenario.Parameters[j].ID })
	metadata, err := normalizeMetadata(scenario.Metadata)
	if err != nil {
		return Scenario{}, err
	}
	scenario.Metadata = metadata
	if err := scenario.Validate(); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}

func CanonicalJSON(source Scenario) ([]byte, error) {
	normalized, err := Normalize(source)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func SemanticDigest(source Scenario) (string, error) {
	normalized, err := Normalize(source)
	if err != nil {
		return "", err
	}
	semantic := struct {
		Triggers   []Trigger   `json:"triggers"`
		Condition  Expr        `json:"condition"`
		Flow       Flow        `json:"flow"`
		Parameters []Parameter `json:"parameters,omitempty"`
	}{
		Triggers:   normalized.Triggers,
		Condition:  normalized.Condition,
		Flow:       normalized.Flow,
		Parameters: normalized.Parameters,
	}
	raw, err := json.Marshal(semantic)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
