package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ComputePlanDigest calculates a deterministic sha256 digest of the compiled execution plan.
func ComputePlanDigest(m *Manifest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("compiler: manifest is nil")
	}

	payload := struct {
		ScenarioID          string               `json:"scenarioId"`
		SemanticDigest      string               `json:"semanticDigest"`
		CompilerVersion     string               `json:"compilerVersion"`
		CapabilityVersions  map[string]string    `json:"capabilityVersions"`
		SelectedRuntime     Runtime              `json:"selectedRuntime"`
		RequiredPermissions []string             `json:"requiredPermissions"`
		PhysicalResources   []string             `json:"physicalResources"`
		ExternalEffects     []ExternalEffectSpec `json:"externalEffects"`
		SafetyAugmentations []SafetyAugmentation `json:"safetyAugmentations"`
		AxiomPlan           *AxiomPlan           `json:"axiomPlan,omitempty"`
		ADGOPlan            *ADGOPlan            `json:"adgoPlan,omitempty"`
	}{
		ScenarioID:          m.ScenarioID,
		SemanticDigest:      m.SemanticDigest,
		CompilerVersion:     m.CompilerVersion,
		CapabilityVersions:  m.CapabilityVersions,
		SelectedRuntime:     m.SelectedRuntime,
		RequiredPermissions: m.RequiredPermissions,
		PhysicalResources:   m.PhysicalResources,
		ExternalEffects:     m.ExternalEffects,
		SafetyAugmentations: m.SafetyAugmentations,
		AxiomPlan:           m.AxiomPlan,
		ADGOPlan:            m.ADGOPlan,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("compiler: compute plan digest: %w", err)
	}

	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
