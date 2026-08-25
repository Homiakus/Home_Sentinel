package simulator

import (
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type Simulator struct {
	compiler *compiler.Compiler
}

func NewSimulator(comp *compiler.Compiler) *Simulator {
	return &Simulator{compiler: comp}
}

// Simulate runs a pure dry-run headless simulation of the scenario under the specified context and virtual clock.
// It NEVER invokes real physical gateways and records all actions as hypothetical WOULD_EXECUTE effects.
func (s *Simulator) Simulate(scen model.Scenario, ctx SimulationContext, clock Clock) (*SimulationResult, error) {
	if clock == nil {
		clock = NewVirtualClock(ctx.SimulatedTime)
	}

	manifest, diags := s.compiler.Compile(scen)
	_ = manifest
	if diags.HasErrors() {
		return &SimulationResult{
			ScenarioID: string(scen.ID),
			RevisionID: string(scen.RevisionID),
			Mode:       ctx.Mode,
			Passed:     false,
			FinalOutcome: "COMPILATION_FAILED",
			Errors:     []string{diags.Error()},
		}, nil
	}

	start := clock.Now()
	res := &SimulationResult{
		ScenarioID: string(scen.ID),
		RevisionID: string(scen.RevisionID),
		Mode:       ctx.Mode,
		StartTime:  start,
		Passed:     true,
	}

	// Build runtime evaluation environment
	env := make(EvalEnv)
	for k, v := range ctx.HomeState {
		env[k] = v
		env["state."+k] = v
	}
	for k, v := range ctx.TriggerEvent {
		env["trigger."+k] = v
	}
	for _, p := range scen.Parameters {
		if p.Default != nil {
			env["parameter."+p.ID] = *p.Default
		}
	}

	// 1. Evaluate Triggers
	triggerMatched := false
	matchedTriggerID := ""
	for _, trig := range scen.Triggers {
		// Filter evaluation
		if !trig.Filter.IsZero() {
			pass, err := EvaluateBool(trig.Filter, env)
			if err != nil {
				res.Traces = append(res.Traces, TraceStep{
					StepID:      trig.ID,
					Kind:        "trigger_filter",
					Outcome:     "ERROR",
					Explanation: fmt.Sprintf("Trigger filter evaluation error: %v", err),
					Timestamp:   clock.Now(),
				})
				continue
			}
			if !pass {
				res.Traces = append(res.Traces, TraceStep{
					StepID:      trig.ID,
					Kind:        "trigger_filter",
					Outcome:     "FILTER_MISMATCH",
					Explanation: fmt.Sprintf("Trigger %q filter evaluated to false", trig.ID),
					Timestamp:   clock.Now(),
				})
				continue
			}
		}

		triggerMatched = true
		matchedTriggerID = trig.ID
		res.Traces = append(res.Traces, TraceStep{
			StepID:      trig.ID,
			Kind:        "trigger",
			Outcome:     "MATCH",
			Explanation: fmt.Sprintf("Trigger %q matched event payload", trig.ID),
			Timestamp:   clock.Now(),
		})
		break
	}

	if !triggerMatched {
		res.EndTime = clock.Now()
		res.SimulatedDuration = res.EndTime.Sub(start)
		res.FinalOutcome = "TRIGGER_NOT_MATCHED"
		return res, nil
	}

	// 2. Evaluate Condition
	if !scen.Condition.IsZero() {
		condPass, err := EvaluateBool(scen.Condition, env)
		if err != nil {
			res.Passed = false
			res.Errors = append(res.Errors, fmt.Sprintf("condition evaluation error: %v", err))
			res.Traces = append(res.Traces, TraceStep{
				StepID:      "condition",
				Kind:        "condition",
				Outcome:     "ERROR",
				Explanation: fmt.Sprintf("Condition evaluation failed: %v", err),
				Timestamp:   clock.Now(),
			})
			res.FinalOutcome = "CONDITION_ERROR"
			return res, nil
		}

		if !condPass {
			res.Traces = append(res.Traces, TraceStep{
				StepID:      "condition",
				Kind:        "condition",
				Outcome:     "CONDITION_FALSE",
				Explanation: "Top-level scenario condition evaluated to false; skipping flow execution",
				Timestamp:   clock.Now(),
			})
			res.EndTime = clock.Now()
			res.SimulatedDuration = res.EndTime.Sub(start)
			res.FinalOutcome = "CONDITION_SKIPPED"
			return res, nil
		}

		res.Traces = append(res.Traces, TraceStep{
			StepID:      "condition",
			Kind:        "condition",
			Outcome:     "CONDITION_TRUE",
			Explanation: "Top-level scenario condition passed",
			Timestamp:   clock.Now(),
		})
	}

	// 3. Execute Flow Steps (using System Graph augmented nodes for safety verification)
	var executeFlow func(flow model.Flow) bool
	executeFlow = func(flow model.Flow) bool {
		for _, step := range flow.Steps {
			stepID := string(step.ID)

			switch step.Kind {
			case model.StepAction:
				// Check if action is high risk (e.g. unlock)
				isUnlock := strings.Contains(strings.ToLower(step.Action.Capability.ID), "unlock")
				isSiren := strings.Contains(strings.ToLower(step.Action.Capability.ID), "siren")

				if isUnlock {
					// Simulate mandatory human approval check
					approved := true
					if ctx.HumanApprovals != nil {
						if app, ok := ctx.HumanApprovals[stepID]; ok {
							approved = app
						}
					}
					if !approved {
						res.Traces = append(res.Traces, TraceStep{
							StepID:      "safety_approval_" + stepID,
							Kind:        "human_approval",
							SystemOwned: true,
							Outcome:     "REJECTED",
							Explanation: "Mandatory human approval rejected unlock request",
							Timestamp:   clock.Now(),
						})
						res.FinalOutcome = "APPROVAL_REJECTED"
						return false
					}

					res.Traces = append(res.Traces, TraceStep{
						StepID:      "safety_approval_" + stepID,
						Kind:        "human_approval",
						SystemOwned: true,
						Outcome:     "APPROVED",
						Explanation: "Simulated administrator approval granted for high-risk action",
						Timestamp:   clock.Now(),
					})
				}

				// Evaluate arguments
				argsMap := make(map[string]any)
				for k, expr := range step.Action.Arguments {
					val, err := Evaluate(expr, env)
					if err == nil {
						var v any
						_ = val.Unmarshal(&v)
						argsMap[k] = v
					}
				}

				rKey := ""
				if step.Action.Capability.Entity != nil {
					rKey = fmt.Sprintf("%s:%s", step.Action.Capability.Entity.Kind, step.Action.Capability.Entity.ID)
				}

				risk := model.RiskLow
				if isUnlock {
					risk = model.RiskCritical
				} else if isSiren {
					risk = model.RiskHigh
				}

				// Record WOULD_EXECUTE
				res.HypotheticalEffects = append(res.HypotheticalEffects, HypotheticalEffect{
					CapabilityID: step.Action.Capability.ID,
					ResourceKey:  rKey,
					Risk:         risk,
					Arguments:    argsMap,
					Action:       "WOULD_EXECUTE",
					Timestamp:    clock.Now(),
				})

				res.Traces = append(res.Traces, TraceStep{
					StepID:      stepID,
					Kind:        "action",
					Outcome:     "WOULD_EXECUTE",
					Inputs:      argsMap,
					Explanation: fmt.Sprintf("Simulated action %q on resource %q (Risk: %s)", step.Action.Capability.ID, rKey, risk),
					Timestamp:   clock.Now(),
				})

			case model.StepWait:
				clock.Advance(step.Wait.Duration)
				res.Traces = append(res.Traces, TraceStep{
					StepID:      stepID,
					Kind:        "wait",
					Outcome:     "WAITED",
					Explanation: fmt.Sprintf("Advanced virtual clock by %v", step.Wait.Duration),
					Timestamp:   clock.Now(),
				})

			case model.StepHumanApproval:
				approved := true
				if ctx.HumanApprovals != nil {
					if app, ok := ctx.HumanApprovals[stepID]; ok {
						approved = app
					}
				}
				if !approved {
					res.Traces = append(res.Traces, TraceStep{
						StepID:      stepID,
						Kind:        "human_approval",
						Outcome:     "REJECTED",
						Explanation: fmt.Sprintf("Human approval %q rejected", step.HumanApproval.Prompt),
						Timestamp:   clock.Now(),
					})
					res.FinalOutcome = "APPROVAL_REJECTED"
					return false
				}
				res.Traces = append(res.Traces, TraceStep{
					StepID:      stepID,
					Kind:        "human_approval",
					Outcome:     "APPROVED",
					Explanation: fmt.Sprintf("Human approval %q approved", step.HumanApproval.Prompt),
					Timestamp:   clock.Now(),
				})

			case model.StepIf:
				condPass, err := EvaluateBool(step.If.Condition, env)
				if err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("step %q if condition error: %v", stepID, err))
					return false
				}
				if condPass {
					res.Traces = append(res.Traces, TraceStep{
						StepID:      stepID,
						Kind:        "if",
						Outcome:     "BRANCH_THEN",
						Explanation: "If condition evaluated to true; executing THEN branch",
						Timestamp:   clock.Now(),
					})
					if !executeFlow(step.If.Then) {
						return false
					}
				} else if step.If.Else != nil {
					res.Traces = append(res.Traces, TraceStep{
						StepID:      stepID,
						Kind:        "if",
						Outcome:     "BRANCH_ELSE",
						Explanation: "If condition evaluated to false; executing ELSE branch",
						Timestamp:   clock.Now(),
					})
					if !executeFlow(*step.If.Else) {
						return false
					}
				}

			case model.StepStop:
				res.Traces = append(res.Traces, TraceStep{
					StepID:      stepID,
					Kind:        "stop",
					Outcome:     string(step.Stop.Outcome),
					Explanation: fmt.Sprintf("Flow explicitly stopped with outcome %q", step.Stop.Outcome),
					Timestamp:   clock.Now(),
				})
				return false
			}
		}
		return true
	}

	executeFlow(scen.Flow)
	_ = matchedTriggerID

	res.EndTime = clock.Now()
	res.SimulatedDuration = res.EndTime.Sub(start)
	if res.FinalOutcome == "" {
		res.FinalOutcome = "COMPLETED"
	}

	return res, nil
}
