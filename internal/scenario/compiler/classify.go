package compiler

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type ClassificationResult struct {
	Runtime Runtime
	Reasons []string
}

// ClassifyRuntime determines whether a scenario can execute in Axiom (fast reactive FSM)
// or requires ADGO (durable multi-step workflow with state persistence).
func ClassifyRuntime(s model.Scenario, temporalReqs []TemporalRequirement, safety SafetyAnalysisResult, resolved ResolvedCapabilities) ClassificationResult {
	var reasons []string

	// Check safety requirements
	if safety.RequiresADGO {
		reasons = append(reasons, safety.ADGOReasons...)
	}

	// Check temporal requirements
	for _, req := range temporalReqs {
		if req.Durable {
			reasons = append(reasons, fmt.Sprintf("durable temporal requirement: %s (%v)", req.Kind, req.Duration))
		}
	}

	// Check flow steps complexity
	var inspectFlow func(flow model.Flow)
	inspectFlow = func(flow model.Flow) {
		if len(flow.Steps) > 2 {
			reasons = append(reasons, fmt.Sprintf("flow contains %d sequential steps (> 2 steps requires durable state continuation)", len(flow.Steps)))
		}
		for _, step := range flow.Steps {
			switch step.Kind {
			case model.StepWait:
				reasons = append(reasons, fmt.Sprintf("explicit wait step %q (%v) requires durable timer", step.ID, step.Wait.Duration))
			case model.StepHumanApproval:
				reasons = append(reasons, fmt.Sprintf("human approval step %q requires durable operator suspension", step.ID))
			case model.StepParallel, model.StepJoin:
				reasons = append(reasons, fmt.Sprintf("parallel branching step %q requires durable fork/join coordination", step.ID))
			case model.StepSubflow:
				reasons = append(reasons, fmt.Sprintf("subflow step %q requires durable child workflow execution", step.ID))
			case model.StepSwitch:
				reasons = append(reasons, fmt.Sprintf("multi-branch switch step %q requires workflow decision graph", step.ID))
			case model.StepAction:
				if step.Action.Retry != nil && step.Action.Retry.MaxAttempts > 1 {
					reasons = append(reasons, fmt.Sprintf("action step %q with retry policy (%d attempts) requires durable activity execution", step.ID, step.Action.Retry.MaxAttempts))
				}
				if desc, ok := resolved.Descriptors[step.Action.Capability.ID]; ok {
					if desc.Risk == model.RiskHigh || desc.Risk == model.RiskCritical {
						reasons = append(reasons, fmt.Sprintf("action %q has risk %s requiring durable execution and auditing", desc.ID, desc.Risk))
					}
					if desc.ExternalEffect {
						reasons = append(reasons, fmt.Sprintf("action %q has external physical effect requiring invocation gateway tracking", desc.ID))
					}
				}
			}
			if step.If != nil {
				inspectFlow(step.If.Then)
				if step.If.Else != nil {
					inspectFlow(*step.If.Else)
				}
			}
		}
	}
	inspectFlow(s.Flow)

	// Deduplicate reasons
	seen := make(map[string]struct{})
	var uniqueReasons []string
	for _, r := range reasons {
		if _, exists := seen[r]; !exists {
			seen[r] = struct{}{}
			uniqueReasons = append(uniqueReasons, r)
		}
	}

	if len(uniqueReasons) > 0 {
		return ClassificationResult{
			Runtime: RuntimeADGO,
			Reasons: uniqueReasons,
		}
	}

	return ClassificationResult{
		Runtime: RuntimeAxiom,
		Reasons: []string{"stateless reactive trigger-action flow without durable waits, high risk, or complex branching"},
	}
}
