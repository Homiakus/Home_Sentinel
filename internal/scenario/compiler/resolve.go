package compiler

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type ResolvedCapabilities struct {
	Descriptors         map[string]capability.Descriptor
	ReferencedEntities  []model.EntityRef
	RequiredPermissions []string
	PhysicalResources   []string
	CapabilityVersions  map[string]string
}

// ResolveCapabilities validates that all referenced capabilities exist in the registry and are compatible.
func ResolveCapabilities(s model.Scenario, reg *capability.Registry) (ResolvedCapabilities, Diagnostics) {
	var diags Diagnostics
	res := ResolvedCapabilities{
		Descriptors:        make(map[string]capability.Descriptor),
		CapabilityVersions: make(map[string]string),
	}

	if reg == nil {
		return res, diags
	}

	seenPerms := make(map[string]struct{})
	seenResources := make(map[string]struct{})
	seenEntities := make(map[string]struct{})

	// Check triggers
	for i, trig := range s.Triggers {
		if trig.Capability.ID == "" {
			continue
		}
		path := fmt.Sprintf("triggers[%d].capability", i)
		desc, found := reg.ResolveCompatible(trig.Capability.ID, trig.Capability.Version)
		if !found {
			diags = append(diags, Diagnostic{
				Code:     CodeCapabilityNotFound,
				Severity: SeverityError,
				Path:     path,
				Message:  fmt.Sprintf("trigger capability %s@%s not found in registry", trig.Capability.ID, trig.Capability.Version),
				Hint:     "Verify capability ID and compatible version in Capability Registry",
			})
			continue
		}
		res.Descriptors[desc.ID] = desc
		res.CapabilityVersions[desc.ID] = desc.Version
		if desc.Permission != "" {
			if _, ok := seenPerms[string(desc.Permission)]; !ok {
				seenPerms[string(desc.Permission)] = struct{}{}
				res.RequiredPermissions = append(res.RequiredPermissions, string(desc.Permission))
			}
		}
		if trig.Capability.Entity != nil && trig.Capability.Entity.ID != "" {
			entKey := trig.Capability.Entity.Kind + ":" + trig.Capability.Entity.ID
			if _, ok := seenEntities[entKey]; !ok {
				seenEntities[entKey] = struct{}{}
				res.ReferencedEntities = append(res.ReferencedEntities, *trig.Capability.Entity)
			}
		}
	}

	// Check actions in flow
	var walkFlow func(path string, flow model.Flow)
	walkFlow = func(path string, flow model.Flow) {
		for i, step := range flow.Steps {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, i)
			if step.Action != nil {
				capRef := step.Action.Capability
				actionPath := stepPath + ".action.capability"
				desc, found := reg.ResolveCompatible(capRef.ID, capRef.Version)
				if !found {
					diags = append(diags, Diagnostic{
						Code:     CodeCapabilityNotFound,
						Severity: SeverityError,
						Path:     actionPath,
						Message:  fmt.Sprintf("action capability %s@%s not found in registry", capRef.ID, capRef.Version),
						Hint:     "Ensure capability is registered and version constraint is satisfied",
					})
					continue
				}
				res.Descriptors[desc.ID] = desc
				res.CapabilityVersions[desc.ID] = desc.Version

				if desc.Permission != "" {
					if _, ok := seenPerms[string(desc.Permission)]; !ok {
						seenPerms[string(desc.Permission)] = struct{}{}
						res.RequiredPermissions = append(res.RequiredPermissions, string(desc.Permission))
					}
				}

				if capRef.Entity != nil && capRef.Entity.ID != "" {
					entKey := capRef.Entity.Kind + ":" + capRef.Entity.ID
					if _, ok := seenEntities[entKey]; !ok {
						seenEntities[entKey] = struct{}{}
						res.ReferencedEntities = append(res.ReferencedEntities, *capRef.Entity)
					}
					resourceKey := fmt.Sprintf("%s:%s", capRef.Entity.Kind, capRef.Entity.ID)
					if _, ok := seenResources[resourceKey]; !ok {
						seenResources[resourceKey] = struct{}{}
						res.PhysicalResources = append(res.PhysicalResources, resourceKey)
					}
				}

				if desc.Health == capability.HealthOffline {
					diags = append(diags, Diagnostic{
						Code:     CodeCapabilityUnavailable,
						Severity: SeverityWarning,
						Path:     actionPath,
						Message:  fmt.Sprintf("capability %s is currently offline; executions may fail or queue", desc.ID),
					})
				}
			}
			if step.If != nil {
				walkFlow(stepPath+".if.then", step.If.Then)
				if step.If.Else != nil {
					walkFlow(stepPath+".if.else", *step.If.Else)
				}
			}
			if step.Switch != nil {
				for j, c := range step.Switch.Cases {
					walkFlow(fmt.Sprintf("%s.switch.cases[%d].flow", stepPath, j), c.Flow)
				}
				if step.Switch.Default != nil {
					walkFlow(stepPath+".switch.default", *step.Switch.Default)
				}
			}
			if step.Parallel != nil {
				for j, b := range step.Parallel.Branches {
					walkFlow(fmt.Sprintf("%s.parallel.branches[%d]", stepPath, j), b)
				}
			}
		}
	}

	walkFlow("flow", s.Flow)
	return res, diags
}
