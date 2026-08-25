package compiler

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

const CompilerVersion = "1.0.0"

type Compiler struct {
	registry *capability.Registry
}

func NewCompiler(registry *capability.Registry) *Compiler {
	return &Compiler{registry: registry}
}

// Compile executes all compiler passes in order and returns the compiled Manifest and any diagnostics.
func (c *Compiler) Compile(source model.Scenario) (*Manifest, Diagnostics) {
	var allDiags Diagnostics

	// 1. Normalize
	normalized, err := model.Normalize(source)
	if err != nil {
		allDiags = append(allDiags, Diagnostic{
			Code:     CodeMalformedStructure,
			Severity: SeverityError,
			Path:     "scenario",
			Message:  fmt.Sprintf("normalization failed: %v", err),
		})
		return nil, allDiags
	}

	// 2. Semantic Digest
	semanticDigest, err := model.SemanticDigest(normalized)
	if err != nil {
		allDiags = append(allDiags, Diagnostic{
			Code:     CodeCompilerInvariantBroken,
			Severity: SeverityError,
			Path:     "scenario",
			Message:  fmt.Sprintf("semantic digest computation failed: %v", err),
		})
		return nil, allDiags
	}

	// 3. Build Type Environment
	env, envDiags := BuildTypeEnvironment(normalized, c.registry)
	allDiags = append(allDiags, envDiags...)

	// 4. Resolve Capabilities
	resolved, resolveDiags := ResolveCapabilities(normalized, c.registry)
	allDiags = append(allDiags, resolveDiags...)

	// 5. Type Check & Schema Validation
	typeDiags := TypeCheckScenario(normalized, env, resolved)
	allDiags = append(allDiags, typeDiags...)

	// 6. Temporal Analysis
	temporalReqs, tempDiags := AnalyzeTemporal(normalized)
	allDiags = append(allDiags, tempDiags...)

	// 7. Safety Compiler
	safetyRes, safetyDiags := ApplySafetyPass(normalized, resolved)
	allDiags = append(allDiags, safetyDiags...)

	// 8. Static Conflict Analysis
	staticDiags := StaticConflictAnalysis(normalized, resolved)
	allDiags = append(allDiags, staticDiags...)

	// 9. Runtime Classification
	classRes := ClassifyRuntime(normalized, temporalReqs, safetyRes, resolved)

	// 10. Lowering
	var axiomPlan *AxiomPlan
	var adgoPlan *ADGOPlan

	if classRes.Runtime == RuntimeAxiom {
		var axDiags Diagnostics
		axiomPlan, axDiags = LowerToAxiom(normalized)
		allDiags = append(allDiags, axDiags...)
	} else {
		var adgoDiags Diagnostics
		adgoPlan, adgoDiags = LowerToADGO(normalized, safetyRes, resolved)
		allDiags = append(allDiags, adgoDiags...)
	}

	// 11. Manifest Assembly
	manifest := &Manifest{
		ScenarioID:           string(normalized.ID),
		RevisionID:           string(normalized.RevisionID),
		SemanticDigest:       semanticDigest,
		CompilerVersion:      CompilerVersion,
		CapabilityVersions:   resolved.CapabilityVersions,
		SelectedRuntime:      classRes.Runtime,
		RuntimeReasons:       classRes.Reasons,
		RequiredPermissions:  resolved.RequiredPermissions,
		ReferencedEntities:   resolved.ReferencedEntities,
		PhysicalResources:    resolved.PhysicalResources,
		ExternalEffects:      safetyRes.ExternalEffects,
		SafetyAugmentations:  safetyRes.Augmentations,
		TemporalRequirements: temporalReqs,
		Migration: MigrationInfo{
			CompatibleFromVersion: "1.0.0",
		},
		UserGraph:   safetyRes.UserGraph,
		SystemGraph: safetyRes.SystemGraph,
		AxiomPlan:   axiomPlan,
		ADGOPlan:    adgoPlan,
		Diagnostics: allDiags,
	}

	// 12. Deterministic Plan Digest
	planDigest, err := ComputePlanDigest(manifest)
	if err != nil {
		allDiags = append(allDiags, Diagnostic{
			Code:     CodeCompilerInvariantBroken,
			Severity: SeverityError,
			Path:     "plan",
			Message:  fmt.Sprintf("plan digest computation failed: %v", err),
		})
	} else {
		manifest.PlanDigest = planDigest
	}

	return manifest, allDiags
}
