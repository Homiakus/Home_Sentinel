package compiler

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

// LowerToADGO generates a durable ADGO workflow plan from the compiled scenario and safety augmentations.
func LowerToADGO(s model.Scenario, safety SafetyAnalysisResult, resolved ResolvedCapabilities) (*ADGOPlan, Diagnostics) {
	var diags Diagnostics

	plan := &ADGOPlan{
		WorkflowID: fmt.Sprintf("adgo_wf_%s_%s", s.ID, s.RevisionID),
		EntryNode:  "entry",
	}

	for _, eff := range safety.ExternalEffects {
		if eff.ResourceKey != "" {
			plan.ResourceKeys = append(plan.ResourceKeys, eff.ResourceKey)
		}
	}

	// Translate System Graph nodes into ADGO executable nodes
	for i, gNode := range safety.SystemGraph.Nodes {
		nodeType := "Activity"
		props := make(map[string]any)

		switch gNode.Kind {
		case "trigger":
			nodeType = "Trigger"
		case "action":
			nodeType = "Activity"
		case "wait":
			nodeType = "Wait"
		case "human_approval":
			nodeType = "Human"
			props["risk"] = gNode.Risk
		case "parallel":
			nodeType = "Fork"
		case "join":
			nodeType = "Join"
		case "subflow":
			nodeType = "Subflow"
		case "if", "decision":
			nodeType = "Decision"
		case "resource_reservation":
			nodeType = "ResourceLock"
		case "read_before_write":
			nodeType = "ReadState"
		case "verify_after_write":
			nodeType = "VerifyWrite"
		case "compensation":
			nodeType = "Compensation"
		case "duration_limit":
			nodeType = "DurationLimit"
		default:
			nodeType = "Activity"
		}

		if len(gNode.Details) > 0 {
			for k, v := range gNode.Details {
				props[k] = v
			}
		}

		plan.Nodes = append(plan.Nodes, ADGONode{
			ID:          gNode.ID,
			Type:        nodeType,
			SystemOwned: gNode.SystemOwned,
			Transitions: gNode.Next,
			Properties:  props,
		})
		_ = i
	}

	return plan, diags
}
