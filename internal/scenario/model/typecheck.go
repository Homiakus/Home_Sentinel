package model

import "fmt"

func ValidateTypes(s Scenario, supplied TypeEnv) error {
	env := make(TypeEnv, len(supplied)+len(s.Parameters))
	for key, value := range supplied {
		env[key] = value
	}
	for _, parameter := range s.Parameters {
		env["parameter."+parameter.ID] = parameter.Type
	}
	if !s.Condition.IsZero() {
		if err := requireBool("condition", s.Condition, env); err != nil {
			return err
		}
	}
	for i := range s.Triggers {
		if !s.Triggers[i].Filter.IsZero() {
			if err := requireBool(fmt.Sprintf("triggers[%d].filter", i), s.Triggers[i].Filter, env); err != nil {
				return err
			}
		}
		for j := range s.Triggers[i].Temporal {
			if s.Triggers[i].Temporal[j].Kind == TemporalUntil {
				if err := requireBool(fmt.Sprintf("triggers[%d].temporal[%d].until", i, j), s.Triggers[i].Temporal[j].Until, env); err != nil {
					return err
				}
			}
		}
	}
	return validateFlowTypes("flow", s.Flow, env)
}

func validateFlowTypes(path string, flow Flow, env TypeEnv) error {
	for i := range flow.Steps {
		step := flow.Steps[i]
		stepPath := fmt.Sprintf("%s.steps[%d]", path, i)
		if step.If != nil {
			if err := requireBool(stepPath+".if.condition", step.If.Condition, env); err != nil {
				return err
			}
			if err := validateFlowTypes(stepPath+".if.then", step.If.Then, env); err != nil {
				return err
			}
			if step.If.Else != nil {
				if err := validateFlowTypes(stepPath+".if.else", *step.If.Else, env); err != nil {
					return err
				}
			}
		}
		if step.Switch != nil {
			for j := range step.Switch.Cases {
				if err := requireBool(fmt.Sprintf("%s.switch.cases[%d].when", stepPath, j), step.Switch.Cases[j].When, env); err != nil {
					return err
				}
				if err := validateFlowTypes(fmt.Sprintf("%s.switch.cases[%d].flow", stepPath, j), step.Switch.Cases[j].Flow, env); err != nil {
					return err
				}
			}
			if step.Switch.Default != nil {
				if err := validateFlowTypes(stepPath+".switch.default", *step.Switch.Default, env); err != nil {
					return err
				}
			}
		}
		if step.Parallel != nil {
			for j := range step.Parallel.Branches {
				if err := validateFlowTypes(fmt.Sprintf("%s.parallel.branches[%d]", stepPath, j), step.Parallel.Branches[j], env); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func requireBool(path string, expr Expr, env TypeEnv) error {
	typ, err := CheckExpr(expr, env)
	if err != nil {
		return fmt.Errorf("scenario: %s: %w", path, err)
	}
	if typ.Kind != TypeBool {
		return fmt.Errorf("scenario: %s must evaluate to bool, got %q", path, typ.Kind)
	}
	return nil
}
