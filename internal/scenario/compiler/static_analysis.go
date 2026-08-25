package compiler

import (
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

// StaticConflictAnalysis performs comprehensive pre-publish sanity and safety checks.
func StaticConflictAnalysis(s model.Scenario, resolved ResolvedCapabilities) Diagnostics {
	var diags Diagnostics

	// 1. Self recursion check: a scenario cannot call itself as a subflow
	var checkSelfRecursion func(path string, flow model.Flow)
	checkSelfRecursion = func(path string, flow model.Flow) {
		for i, step := range flow.Steps {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, i)
			if step.Subflow != nil {
				if string(step.Subflow.ScenarioID) == string(s.ID) {
					diags = append(diags, Diagnostic{
						Code:     CodeSelfRecursion,
						Severity: SeverityError,
						Path:     stepPath + ".subflow.scenarioId",
						Message:  fmt.Sprintf("scenario %q directly invokes itself as a subflow (infinite recursion)", s.ID),
						Hint:     "Remove recursive subflow invocation",
					})
				}
			}
			if step.If != nil {
				checkSelfRecursion(stepPath+".if.then", step.If.Then)
				if step.If.Else != nil {
					checkSelfRecursion(stepPath+".if.else", *step.If.Else)
				}
			}
			if step.Switch != nil {
				for j, c := range step.Switch.Cases {
					checkSelfRecursion(fmt.Sprintf("%s.switch.cases[%d].flow", stepPath, j), c.Flow)
				}
				if step.Switch.Default != nil {
					checkSelfRecursion(stepPath+".switch.default", *step.Switch.Default)
				}
			}
			if step.Parallel != nil {
				for j, b := range step.Parallel.Branches {
					checkSelfRecursion(fmt.Sprintf("%s.parallel.branches[%d]", stepPath, j), b)
				}
			}
		}
	}
	checkSelfRecursion("flow", s.Flow)

	// 2. Resource state conflict analysis: detect immediate contradictory actions on the same resource
	// e.g. turning light ON and OFF in parallel branches or adjacent steps without delay
	resourceActions := make(map[string][]string) // resourceKey -> list of capability IDs

	var collectResourceActions func(flow model.Flow)
	collectResourceActions = func(flow model.Flow) {
		for _, step := range flow.Steps {
			if step.Action != nil && step.Action.Capability.Entity != nil {
				resKey := fmt.Sprintf("%s:%s", step.Action.Capability.Entity.Kind, step.Action.Capability.Entity.ID)
				resourceActions[resKey] = append(resourceActions[resKey], step.Action.Capability.ID)
			}
			if step.If != nil {
				collectResourceActions(step.If.Then)
				if step.If.Else != nil {
					collectResourceActions(*step.If.Else)
				}
			}
			if step.Parallel != nil {
				// In parallel branches, contradictory operations on same resource are a severe error
				branchRes := make(map[string]string)
				for _, b := range step.Parallel.Branches {
					for _, bs := range b.Steps {
						if bs.Action != nil && bs.Action.Capability.Entity != nil {
							rKey := fmt.Sprintf("%s:%s", bs.Action.Capability.Entity.Kind, bs.Action.Capability.Entity.ID)
							if prevCap, exists := branchRes[rKey]; exists && prevCap != bs.Action.Capability.ID {
								diags = append(diags, Diagnostic{
									Code:     CodeOppositeDesiredState,
									Severity: SeverityError,
									Path:     "flow",
									Message:  fmt.Sprintf("parallel branches contain conflicting actions on resource %q (%s vs %s)", rKey, prevCap, bs.Action.Capability.ID),
									Hint:     "Serialize conflicting actions or coordinate branch executions",
								})
							}
							branchRes[rKey] = bs.Action.Capability.ID
						}
					}
				}
			}
		}
	}
	collectResourceActions(s.Flow)

	// 3. Unreachable steps check: after a StopStep, following steps in the same sequential flow are unreachable
	var checkUnreachable func(path string, flow model.Flow)
	checkUnreachable = func(path string, flow model.Flow) {
		stopped := false
		for i, step := range flow.Steps {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, i)
			if stopped {
				diags = append(diags, Diagnostic{
					Code:     CodeUnreachableStep,
					Severity: SeverityWarning,
					Path:     stepPath,
					Message:  fmt.Sprintf("step %q is unreachable because a preceding step stopped flow execution", step.ID),
					Hint:     "Remove or reorder unreachable steps",
				})
			}
			if step.Kind == model.StepStop {
				stopped = true
			}
			if step.If != nil {
				checkUnreachable(stepPath+".if.then", step.If.Then)
				if step.If.Else != nil {
					checkUnreachable(stepPath+".if.else", *step.If.Else)
				}
			}
			if step.Switch != nil {
				for j, c := range step.Switch.Cases {
					checkUnreachable(fmt.Sprintf("%s.switch.cases[%d].flow", stepPath, j), c.Flow)
				}
				if step.Switch.Default != nil {
					checkUnreachable(stepPath+".switch.default", *step.Switch.Default)
				}
			}
		}
	}
	checkUnreachable("flow", s.Flow)

	// 4. Check for scenario self-trigger loop (e.g. trigger on siren on and action turns siren on)
	for _, trig := range s.Triggers {
		for resKey, caps := range resourceActions {
			for _, capID := range caps {
				if trig.Capability.ID != "" && strings.EqualFold(trig.Capability.ID, capID) {
					diags = append(diags, Diagnostic{
						Code:     CodeCircularDependency,
						Severity: SeverityWarning,
						Path:     "triggers",
						Message:  fmt.Sprintf("potential feedback loop: trigger %q matches action capability %q on resource %s", trig.Capability.ID, capID, resKey),
						Hint:     "Ensure conditions or state filters prevent continuous re-triggering",
					})
				}
			}
		}
	}

	return diags
}
