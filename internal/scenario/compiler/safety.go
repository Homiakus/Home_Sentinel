package compiler

import (
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type SafetyAnalysisResult struct {
	Augmentations   []SafetyAugmentation
	ExternalEffects []ExternalEffectSpec
	UserGraph       GraphRepresentation
	SystemGraph     GraphRepresentation
	RequiresADGO    bool
	ADGOReasons     []string
}

const (
	MaxSirenDuration = 5 * time.Minute
	DefaultActionTimeout = 30 * time.Second
)

// ApplySafetyPass inspects all physical and high-risk actions, ensures mandatory gates (human approvals,
// single-writer resource locks, read-before-write, verify-after-write, maximum timeouts, compensation),
// and builds separate User Intent Graph and System Graph representations.
func ApplySafetyPass(s model.Scenario, resolved ResolvedCapabilities) (SafetyAnalysisResult, Diagnostics) {
	var diags Diagnostics
	res := SafetyAnalysisResult{
		UserGraph:   GraphRepresentation{EntryNode: "entry"},
		SystemGraph: GraphRepresentation{EntryNode: "entry"},
	}

	// 1. Build User Graph
	userNodes := []GraphNode{
		{ID: "entry", Kind: "trigger", Title: "Triggers", Risk: model.RiskLow},
	}
	systemNodes := []GraphNode{
		{ID: "entry", Kind: "trigger", Title: "Triggers", Risk: model.RiskLow},
	}

	lastUserNodeID := "entry"
	lastSystemNodeID := "entry"

	var walkFlow func(flow model.Flow, prefix string)
	walkFlow = func(flow model.Flow, prefix string) {
		for i, step := range flow.Steps {
			stepID := string(step.ID)
			if stepID == "" {
				stepID = fmt.Sprintf("%s_step_%d", prefix, i)
			}

			if step.Action != nil {
				desc, hasDesc := resolved.Descriptors[step.Action.Capability.ID]
				risk := model.RiskLow
				if hasDesc {
					risk = desc.Risk
				}

				// Check risk level & external effects
				isUnlock := strings.Contains(strings.ToLower(step.Action.Capability.ID), "unlock") ||
					strings.Contains(strings.ToLower(step.Action.Capability.ID), "door.open")
				isSiren := strings.Contains(strings.ToLower(step.Action.Capability.ID), "siren") ||
					strings.Contains(strings.ToLower(step.Action.Capability.ID), "alarm")

				if isUnlock {
					risk = model.RiskCritical
				}

				resourceKey := ""
				if step.Action.Capability.Entity != nil {
					resourceKey = fmt.Sprintf("%s:%s", step.Action.Capability.Entity.Kind, step.Action.Capability.Entity.ID)
				}

				// Record User Graph Node
				userNode := GraphNode{
					ID:    stepID,
					Kind:  "action",
					Title: fmt.Sprintf("Action: %s", step.Action.Capability.ID),
					Risk:  risk,
				}
				userNodes = append(userNodes, userNode)
				if len(userNodes) > 1 {
					userNodes[len(userNodes)-2].Next = []string{stepID}
				}
				lastUserNodeID = stepID

				// Safety Augmentations for System Graph
				if isUnlock {
					res.RequiresADGO = true
					res.ADGOReasons = append(res.ADGOReasons, "critical physical action (door unlock) requires human approval gate and resource locking")

					// 1. Mandatory Human Approval gate
					approvalID := fmt.Sprintf("system_safety_approval_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:          approvalID,
						Kind:        "human_approval",
						Title:       "Mandatory Admin Approval",
						SystemOwned: true,
						Risk:        model.RiskCritical,
					})
					systemNodes[len(systemNodes)-2].Next = []string{approvalID}

					res.Augmentations = append(res.Augmentations, SafetyAugmentation{
						NodeID:      approvalID,
						SystemOwned: true,
						Kind:        "human_approval",
						Description: "Mandatory human administrator confirmation required before unlock",
						Risk:        model.RiskCritical,
						Reason:      "Safety policy forbids autonomous door unlock without interactive confirmation",
					})

					// 2. Resource lock node
					lockID := fmt.Sprintf("system_safety_lock_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:          lockID,
						Kind:        "resource_reservation",
						Title:       fmt.Sprintf("Reserve Resource: %s", resourceKey),
						SystemOwned: true,
						Risk:        model.RiskHigh,
					})
					systemNodes[len(systemNodes)-2].Next = []string{lockID}

					res.Augmentations = append(res.Augmentations, SafetyAugmentation{
						NodeID:      lockID,
						SystemOwned: true,
						Kind:        "resource_reservation",
						Description: fmt.Sprintf("Single-writer reservation for %s", resourceKey),
						Risk:        model.RiskHigh,
						Reason:      "Prevent concurrent conflicting operations on door lock",
					})

					// 3. Read current state
					readID := fmt.Sprintf("system_safety_read_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:          readID,
						Kind:        "read_before_write",
						Title:       "Read Current Door State",
						SystemOwned: true,
						Risk:        model.RiskLow,
					})
					systemNodes[len(systemNodes)-2].Next = []string{readID}

					// 4. Action node
					actionNodeID := fmt.Sprintf("system_action_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:    actionNodeID,
						Kind:  "action",
						Title: fmt.Sprintf("Desired Unlock: %s", step.Action.Capability.ID),
						Risk:  model.RiskCritical,
					})
					systemNodes[len(systemNodes)-2].Next = []string{actionNodeID}

					// 5. Verify after write
					verifyID := fmt.Sprintf("system_safety_verify_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:          verifyID,
						Kind:        "verify_after_write",
						Title:       "Verify Unlock Confirmed",
						SystemOwned: true,
						Risk:        model.RiskLow,
					})
					systemNodes[len(systemNodes)-2].Next = []string{verifyID}

					res.Augmentations = append(res.Augmentations, SafetyAugmentation{
						NodeID:      verifyID,
						SystemOwned: true,
						Kind:        "verify_write",
						Description: "Post-write status verification via telemetry readback",
						Risk:        model.RiskLow,
						Reason:      "Ensure physical actuator confirmed desired state",
					})

					lastSystemNodeID = verifyID

					res.ExternalEffects = append(res.ExternalEffects, ExternalEffectSpec{
						CapabilityID:     step.Action.Capability.ID,
						ResourceKey:      resourceKey,
						Permission:       "door:unlock",
						Risk:             model.RiskCritical,
						Idempotency:      "required",
						Timeout:          15 * time.Second,
						MaxRetries:       2,
						ReadBeforeWrite:  true,
						VerifyAfterWrite: true,
						Reconciliation:   true,
						Compensation:     "reconcile",
					})
				} else if isSiren {
					res.RequiresADGO = true
					res.ADGOReasons = append(res.ADGOReasons, "acoustic physical action (siren) requires duration clamp and ensure-disabled compensation")

					// Duration clamping check
					durLimitID := fmt.Sprintf("system_safety_duration_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:          durLimitID,
						Kind:        "duration_limit",
						Title:       fmt.Sprintf("Clamp Max Siren Duration (%v)", MaxSirenDuration),
						SystemOwned: true,
						Risk:        model.RiskMedium,
					})
					systemNodes[len(systemNodes)-2].Next = []string{durLimitID}

					res.Augmentations = append(res.Augmentations, SafetyAugmentation{
						NodeID:      durLimitID,
						SystemOwned: true,
						Kind:        "duration_limit",
						Description: fmt.Sprintf("Hard clamp siren duration to maximum %v", MaxSirenDuration),
						Risk:        model.RiskMedium,
						Reason:      "Noise pollution and acoustic safety limits",
					})

					// Action node
					actionNodeID := fmt.Sprintf("system_action_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:    actionNodeID,
						Kind:  "action",
						Title: fmt.Sprintf("Sound Siren: %s", step.Action.Capability.ID),
						Risk:  model.RiskHigh,
					})
					systemNodes[len(systemNodes)-2].Next = []string{actionNodeID}

					// Compensation / Ensure disabled
					compID := fmt.Sprintf("system_safety_comp_%s", stepID)
					systemNodes = append(systemNodes, GraphNode{
						ID:          compID,
						Kind:        "compensation",
						Title:       "Ensure Siren Disabled On Finish/Cancel",
						SystemOwned: true,
						Risk:        model.RiskLow,
					})
					systemNodes[len(systemNodes)-2].Next = []string{compID}

					res.Augmentations = append(res.Augmentations, SafetyAugmentation{
						NodeID:      compID,
						SystemOwned: true,
						Kind:        "compensation",
						Description: "Register fail-safe compensation to ensure siren is deactivated",
						Risk:        model.RiskLow,
						Reason:      "Ensure siren does not remain active indefinitely on error or workflow cancellation",
					})

					lastSystemNodeID = compID

					res.ExternalEffects = append(res.ExternalEffects, ExternalEffectSpec{
						CapabilityID:     step.Action.Capability.ID,
						ResourceKey:      resourceKey,
						Permission:       "siren:sound",
						Risk:             model.RiskHigh,
						Idempotency:      "supported",
						Timeout:          MaxSirenDuration,
						MaxRetries:       1,
						ReadBeforeWrite:  false,
						VerifyAfterWrite: true,
						Reconciliation:   true,
						Compensation:     "ensure_disabled",
					})
				} else {
					// Ordinary action
					if hasDesc && desc.ExternalEffect {
						res.ExternalEffects = append(res.ExternalEffects, ExternalEffectSpec{
							CapabilityID:     desc.ID,
							ResourceKey:      resourceKey,
							Permission:       string(desc.Permission),
							Risk:             desc.Risk,
							Idempotency:      string(desc.Idempotency),
							Timeout:          DefaultActionTimeout,
							MaxRetries:       3,
							ReadBeforeWrite:  false,
							VerifyAfterWrite: desc.Verification == capability.VerificationReadback,
							Reconciliation:   desc.Compensation == capability.CompensationReconcile,
							Compensation:     string(desc.Compensation),
						})
					}

					actionNodeID := stepID
					systemNodes = append(systemNodes, GraphNode{
						ID:    actionNodeID,
						Kind:  "action",
						Title: fmt.Sprintf("Action: %s", step.Action.Capability.ID),
						Risk:  risk,
					})
					systemNodes[len(systemNodes)-2].Next = []string{actionNodeID}
					lastSystemNodeID = actionNodeID
				}
			} else if step.HumanApproval != nil {
				res.RequiresADGO = true
				res.ADGOReasons = append(res.ADGOReasons, "workflow contains interactive HumanApproval step")

				node := GraphNode{
					ID:    stepID,
					Kind:  "human_approval",
					Title: fmt.Sprintf("Approval: %s", step.HumanApproval.Prompt),
					Risk:  step.HumanApproval.Risk,
				}
				userNodes = append(userNodes, node)
				userNodes[len(userNodes)-2].Next = []string{stepID}

				systemNodes = append(systemNodes, node)
				systemNodes[len(systemNodes)-2].Next = []string{stepID}
				lastUserNodeID = stepID
				lastSystemNodeID = stepID
			} else if step.Wait != nil {
				res.RequiresADGO = true
				res.ADGOReasons = append(res.ADGOReasons, fmt.Sprintf("workflow contains durable wait (%v)", step.Wait.Duration))

				node := GraphNode{
					ID:    stepID,
					Kind:  "wait",
					Title: fmt.Sprintf("Wait %v", step.Wait.Duration),
					Risk:  model.RiskLow,
				}
				userNodes = append(userNodes, node)
				userNodes[len(userNodes)-2].Next = []string{stepID}

				systemNodes = append(systemNodes, node)
				systemNodes[len(systemNodes)-2].Next = []string{stepID}
				lastUserNodeID = stepID
				lastSystemNodeID = stepID
			} else if step.Subflow != nil {
				res.RequiresADGO = true
				res.ADGOReasons = append(res.ADGOReasons, fmt.Sprintf("workflow invokes subflow %s@v%d", step.Subflow.ScenarioID, step.Subflow.Version))

				node := GraphNode{
					ID:    stepID,
					Kind:  "subflow",
					Title: fmt.Sprintf("Subflow: %s", step.Subflow.ScenarioID),
					Risk:  model.RiskMedium,
				}
				userNodes = append(userNodes, node)
				userNodes[len(userNodes)-2].Next = []string{stepID}

				systemNodes = append(systemNodes, node)
				systemNodes[len(systemNodes)-2].Next = []string{stepID}
				lastUserNodeID = stepID
				lastSystemNodeID = stepID
			} else if step.Parallel != nil {
				res.RequiresADGO = true
				res.ADGOReasons = append(res.ADGOReasons, "workflow contains parallel branching (fork/join)")

				node := GraphNode{
					ID:    stepID,
					Kind:  "parallel",
					Title: "Parallel Branches",
					Risk:  model.RiskLow,
				}
				userNodes = append(userNodes, node)
				userNodes[len(userNodes)-2].Next = []string{stepID}

				systemNodes = append(systemNodes, node)
				systemNodes[len(systemNodes)-2].Next = []string{stepID}
				lastUserNodeID = stepID
				lastSystemNodeID = stepID
			} else if step.If != nil {
				node := GraphNode{
					ID:    stepID,
					Kind:  "if",
					Title: "Decision / Branch",
					Risk:  model.RiskLow,
				}
				userNodes = append(userNodes, node)
				userNodes[len(userNodes)-2].Next = []string{stepID}

				systemNodes = append(systemNodes, node)
				systemNodes[len(systemNodes)-2].Next = []string{stepID}
				lastUserNodeID = stepID
				lastSystemNodeID = stepID

				walkFlow(step.If.Then, stepID+"_then")
				if step.If.Else != nil {
					walkFlow(*step.If.Else, stepID+"_else")
				}
			} else {
				node := GraphNode{
					ID:    stepID,
					Kind:  string(step.Kind),
					Title: string(step.Kind),
					Risk:  model.RiskLow,
				}
				userNodes = append(userNodes, node)
				userNodes[len(userNodes)-2].Next = []string{stepID}

				systemNodes = append(systemNodes, node)
				systemNodes[len(systemNodes)-2].Next = []string{stepID}
				lastUserNodeID = stepID
				lastSystemNodeID = stepID
			}
		}
	}

	walkFlow(s.Flow, "flow")
	_ = lastUserNodeID
	_ = lastSystemNodeID

	res.UserGraph.Nodes = userNodes
	res.SystemGraph.Nodes = systemNodes
	return res, diags
}
