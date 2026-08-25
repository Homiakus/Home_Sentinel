package compiler

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

// LowerToAxiom generates an Axiom reactive FSM execution plan for simple, stateless scenarios.
func LowerToAxiom(s model.Scenario) (*AxiomPlan, Diagnostics) {
	var diags Diagnostics

	if len(s.Triggers) == 0 {
		diags = append(diags, Diagnostic{
			Code:     CodeAxiomUnsupportedFeature,
			Severity: SeverityError,
			Path:     "triggers",
			Message:  "Axiom lowering requires at least one trigger",
		})
		return nil, diags
	}

	plan := &AxiomPlan{
		ModuleID:  fmt.Sprintf("axiom_mod_%s", s.ID),
		TriggerID: s.Triggers[0].ID,
	}

	if !s.Condition.IsZero() {
		cond := s.Condition
		plan.Condition = &cond
	}

	if len(s.Flow.Steps) > 0 && s.Flow.Steps[0].Action != nil {
		plan.ActionCap = s.Flow.Steps[0].Action.Capability.ID
		plan.ActionArgs = make(map[string]any)
		for k, expr := range s.Flow.Steps[0].Action.Arguments {
			if expr.Op == "literal" && expr.Value != nil {
				var val any
				_ = expr.Value.Unmarshal(&val)
				plan.ActionArgs[k] = val
			} else {
				plan.ActionArgs[k] = expr.Ref
			}
		}
	}

	return plan, diags
}
