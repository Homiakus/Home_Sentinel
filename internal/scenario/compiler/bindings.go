package compiler

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

// BuildTypeEnvironment creates the typing environment for a scenario, combining parameters,
// standard home state variables, and resolved trigger/entity schemas.
func BuildTypeEnvironment(s model.Scenario, reg *capability.Registry) (model.TypeEnv, Diagnostics) {
	var diags Diagnostics
	env := make(model.TypeEnv)

	// Standard Sentinel domain state
	env["state.home.mode"] = model.TypeRef{
		Kind: model.TypeEnum,
		Name: "home_mode",
		Enum: []string{"armed_away", "armed_home", "armed_night", "disarmed", "vacation"},
	}
	env["state.home.presence"] = model.TypeRef{Kind: model.TypeBool}
	env["home.mode"] = env["state.home.mode"] // alias for convenience

	// Parameters
	for _, param := range s.Parameters {
		typ, err := param.Type.Normalize()
		if err != nil {
			diags = append(diags, Diagnostic{
				Code:     CodeTypeMismatch,
				Severity: SeverityError,
				Path:     fmt.Sprintf("parameters[%s].type", param.ID),
				Message:  fmt.Sprintf("invalid parameter type: %v", err),
			})
			continue
		}
		env["parameter."+param.ID] = typ
	}

	// Trigger outputs
	for i, trig := range s.Triggers {
		trigPath := fmt.Sprintf("triggers[%d]", i)
		if reg != nil && trig.Capability.ID != "" {
			desc, found := reg.ResolveCompatible(trig.Capability.ID, trig.Capability.Version)
			if !found {
				// We don't report error here as resolve pass will report CodeCapabilityNotFound
			} else {
				for _, field := range desc.Output.Fields {
					env["trigger."+field.Name] = field.Type
					if trig.ID != "" {
						env[fmt.Sprintf("trigger.%s.%s", trig.ID, field.Name)] = field.Type
					}
				}
			}
		}

		// Generic trigger payload defaults
		if _, ok := env["trigger.person.name"]; !ok {
			env["trigger.person.name"] = model.TypeRef{Kind: model.TypeString}
		}
		if _, ok := env["trigger.snapshot"]; !ok {
			env["trigger.snapshot"] = model.TypeRef{Kind: model.TypeArtifactRef}
		}
		if _, ok := env["trigger.confidence"]; !ok {
			env["trigger.confidence"] = model.TypeRef{Kind: model.TypeConfidence, Unit: "ratio"}
		}
		if _, ok := env["trigger.timestamp"]; !ok {
			env["trigger.timestamp"] = model.TypeRef{Kind: model.TypeTimestamp}
		}
		_ = trigPath
	}

	// Entity states from registry
	if reg != nil {
		snap, err := reg.Snapshot()
		if err == nil {
			for _, ent := range snap.Entities {
				for _, capKey := range ent.Capabilities {
					desc, ok := reg.Get(capKey.ID, capKey.Version)
					if ok && desc.Kind == capability.KindState {
						for _, field := range desc.State.Fields {
							env[fmt.Sprintf("device.%s.%s", ent.ID, field.Name)] = field.Type
							env[fmt.Sprintf("state.%s.%s", ent.ID, field.Name)] = field.Type
						}
					}
				}
			}
		}
	}

	return env, diags
}
