package compiler

import (
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

// TypeCheckScenario performs static type checking and capability argument schema validation across the scenario.
func TypeCheckScenario(s model.Scenario, env model.TypeEnv, resolved ResolvedCapabilities) Diagnostics {
	var diags Diagnostics

	// 1. Condition
	if !s.Condition.IsZero() {
		typ, err := model.CheckExpr(s.Condition, env)
		if err != nil {
			diags = append(diags, Diagnostic{
				Code:     CodeTypeMismatch,
				Severity: SeverityError,
				Path:     "condition",
				Message:  fmt.Sprintf("condition type error: %v", err),
			})
		} else if typ.Kind != model.TypeBool {
			diags = append(diags, Diagnostic{
				Code:     CodeTypeMismatch,
				Severity: SeverityError,
				Path:     "condition",
				Message:  fmt.Sprintf("condition must evaluate to bool, got %q", typ.Kind),
			})
		}
	}

	// 2. Triggers
	for i, trig := range s.Triggers {
		trigPath := fmt.Sprintf("triggers[%d]", i)
		if !trig.Filter.IsZero() {
			typ, err := model.CheckExpr(trig.Filter, env)
			if err != nil {
				diags = append(diags, Diagnostic{
					Code:     CodeTypeMismatch,
					Severity: SeverityError,
					Path:     trigPath + ".filter",
					Message:  fmt.Sprintf("filter expression error: %v", err),
				})
			} else if typ.Kind != model.TypeBool {
				diags = append(diags, Diagnostic{
					Code:     CodeTypeMismatch,
					Severity: SeverityError,
					Path:     trigPath + ".filter",
					Message:  fmt.Sprintf("filter must evaluate to bool, got %q", typ.Kind),
				})
			}
		}
	}

	// 3. Flow
	var checkFlow func(path string, flow model.Flow)
	checkFlow = func(path string, flow model.Flow) {
		for i, step := range flow.Steps {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, i)

			if step.Action != nil {
				desc, hasDesc := resolved.Descriptors[step.Action.Capability.ID]

				for argName, argExpr := range step.Action.Arguments {
					argPath := fmt.Sprintf("%s.action.arguments[%s]", stepPath, argName)
					inferredType, err := model.CheckExpr(argExpr, env)
					if err != nil {
						diags = append(diags, Diagnostic{
							Code:     CodeTypeMismatch,
							Severity: SeverityError,
							Path:     argPath,
							Message:  fmt.Sprintf("argument expression error: %v", err),
						})
						continue
					}

					if hasDesc {
						var fieldSchema *capability.FieldSchema
						for _, f := range desc.Input.Fields {
							if f.Name == argName {
								fieldSchema = &f
								break
							}
						}

						if fieldSchema == nil {
							diags = append(diags, Diagnostic{
								Code:     CodeUnknownArgument,
								Severity: SeverityError,
								Path:     argPath,
								Message:  fmt.Sprintf("unknown argument %q for capability %q", argName, desc.ID),
							})
						} else {
							if !fieldSchema.Type.Compatible(inferredType) {
								diags = append(diags, Diagnostic{
									Code:     CodeTypeMismatch,
									Severity: SeverityError,
									Path:     argPath,
									Message:  fmt.Sprintf("argument %q expects %s, got %s", argName, fieldSchema.Type.Kind, inferredType.Kind),
									Hint:     fmt.Sprintf("Cast or provide an expression matching type %s", fieldSchema.Type.Kind),
								})
							}

							// Enum validation
							if len(fieldSchema.Enum) > 0 && strings.ToLower(argExpr.Op) == "literal" && argExpr.Value != nil && argExpr.Value.Type.Kind == model.TypeString {
								var valStr string
								if err := argExpr.Value.Unmarshal(&valStr); err == nil {
									matched := false
									for _, allowed := range fieldSchema.Enum {
										if allowed == valStr {
											matched = true
											break
										}
									}
									if !matched {
										diags = append(diags, Diagnostic{
											Code:     CodeInvalidEnumValue,
											Severity: SeverityError,
											Path:     argPath,
											Message:  fmt.Sprintf("value %q is not one of allowed enum values %v", valStr, fieldSchema.Enum),
										})
									}
								}
							}
						}
					}
				}

				if hasDesc {
					for _, f := range desc.Input.Fields {
						if f.Required && f.Default == nil {
							if _, exists := step.Action.Arguments[f.Name]; !exists {
								diags = append(diags, Diagnostic{
									Code:     CodeMissingRequiredArg,
									Severity: SeverityError,
									Path:     fmt.Sprintf("%s.action.arguments", stepPath),
									Message:  fmt.Sprintf("missing required argument %q for capability %q", f.Name, desc.ID),
								})
							}
						}
					}
				}
			}

			if step.Subflow != nil {
				for argName, argExpr := range step.Subflow.Arguments {
					argPath := fmt.Sprintf("%s.subflow.arguments[%s]", stepPath, argName)
					if _, err := model.CheckExpr(argExpr, env); err != nil {
						diags = append(diags, Diagnostic{
							Code:     CodeTypeMismatch,
							Severity: SeverityError,
							Path:     argPath,
							Message:  fmt.Sprintf("subflow argument %q expression error: %v", argName, err),
						})
					}
				}
			}

			if step.If != nil {
				condPath := stepPath + ".if.condition"
				typ, err := model.CheckExpr(step.If.Condition, env)
				if err != nil {
					diags = append(diags, Diagnostic{
						Code:     CodeTypeMismatch,
						Severity: SeverityError,
						Path:     condPath,
						Message:  fmt.Sprintf("if condition expression error: %v", err),
					})
				} else if typ.Kind != model.TypeBool {
					diags = append(diags, Diagnostic{
						Code:     CodeTypeMismatch,
						Severity: SeverityError,
						Path:     condPath,
						Message:  fmt.Sprintf("if condition must evaluate to bool, got %q", typ.Kind),
					})
				}
				checkFlow(stepPath+".if.then", step.If.Then)
				if step.If.Else != nil {
					checkFlow(stepPath+".if.else", *step.If.Else)
				}
			}

			if step.Switch != nil {
				for j, c := range step.Switch.Cases {
					casePath := fmt.Sprintf("%s.switch.cases[%d].when", stepPath, j)
					typ, err := model.CheckExpr(c.When, env)
					if err != nil {
						diags = append(diags, Diagnostic{
							Code:     CodeTypeMismatch,
							Severity: SeverityError,
							Path:     casePath,
							Message:  fmt.Sprintf("switch case expression error: %v", err),
						})
					} else if typ.Kind != model.TypeBool {
						diags = append(diags, Diagnostic{
							Code:     CodeTypeMismatch,
							Severity: SeverityError,
							Path:     casePath,
							Message:  fmt.Sprintf("switch case condition must evaluate to bool, got %q", typ.Kind),
						})
					}
					checkFlow(fmt.Sprintf("%s.switch.cases[%d].flow", stepPath, j), c.Flow)
				}
				if step.Switch.Default != nil {
					checkFlow(stepPath+".switch.default", *step.Switch.Default)
				}
			}

			if step.Parallel != nil {
				for j, b := range step.Parallel.Branches {
					checkFlow(fmt.Sprintf("%s.parallel.branches[%d]", stepPath, j), b)
				}
			}
		}
	}

	checkFlow("flow", s.Flow)
	return diags
}
