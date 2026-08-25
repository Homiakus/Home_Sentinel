package compiler

import (
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

func setupTestRegistry(t *testing.T) *capability.Registry {
	reg := capability.NewRegistry(nil)

	// Notification action (low risk, safe, no external physical side effects)
	notifDesc, err := capability.NewActionDescriptor("notification.send", "1.0.0", "core", "notify", "alert", "Send Notification", "notification:send")
	if err != nil {
		t.Fatalf("setup notif desc: %v", err)
	}
	notifDesc.Input = capability.Schema{
		Fields: []capability.FieldSchema{
			{Name: "title", Type: model.TypeRef{Kind: model.TypeString}, Required: true},
			{Name: "body", Type: model.TypeRef{Kind: model.TypeString}},
		},
	}
	if err := reg.Register(notifDesc); err != nil {
		t.Fatalf("register notif: %v", err)
	}

	// Door unlock action (critical risk, physical effect)
	doorDesc, err := capability.NewActionDescriptor("door.unlock", "1.0.0", "core", "access", "door", "Unlock Door", "door:unlock")
	if err != nil {
		t.Fatalf("setup door desc: %v", err)
	}
	doorDesc.Risk = model.RiskCritical
	doorDesc.ExternalEffect = true
	doorDesc.Idempotency = capability.IdempotencyRequired
	doorDesc.Verification = capability.VerificationReadback
	doorDesc.Compensation = capability.CompensationReconcile
	doorDesc.EntityKinds = []string{"door"}
	if err := reg.Register(doorDesc); err != nil {
		t.Fatalf("register door: %v", err)
	}

	// Siren sound action (high risk, acoustic physical effect)
	sirenDesc, err := capability.NewActionDescriptor("siren.sound", "1.0.0", "core", "alarm", "siren", "Sound Siren", "siren:sound")
	if err != nil {
		t.Fatalf("setup siren desc: %v", err)
	}
	sirenDesc.Risk = model.RiskHigh
	sirenDesc.ExternalEffect = true
	sirenDesc.Idempotency = capability.IdempotencySupported
	sirenDesc.Verification = capability.VerificationReadback
	sirenDesc.Compensation = capability.CompensationAutomatic
	sirenDesc.EntityKinds = []string{"siren"}
	sirenDesc.Input = capability.Schema{
		Fields: []capability.FieldSchema{
			{Name: "duration", Type: model.TypeRef{Kind: model.TypeDuration, Unit: "ns"}},
		},
	}
	if err := reg.Register(sirenDesc); err != nil {
		t.Fatalf("register siren: %v", err)
	}

	// Camera person trigger
	trigDesc, err := capability.NewTriggerDescriptor("camera.person.detected", "1.0.0", "frigate", "nvr", "vision", "Person Detected", "camera:read")
	if err != nil {
		t.Fatalf("setup trig desc: %v", err)
	}
	trigDesc.Output = capability.Schema{
		Fields: []capability.FieldSchema{
			{Name: "confidence", Type: model.TypeRef{Kind: model.TypeConfidence, Unit: "ratio"}},
			{Name: "person_name", Type: model.TypeRef{Kind: model.TypeString}},
		},
	}
	if err := reg.Register(trigDesc); err != nil {
		t.Fatalf("register trig: %v", err)
	}

	return reg
}

func TestCompiler_AxiomLowering(t *testing.T) {
	reg := setupTestRegistry(t)
	comp := NewCompiler(reg)

	titleVal, _ := model.ValueOf("Motion detected")
	scen := model.Scenario{
		ID:         "light-motion",
		RevisionID: "rev-1",
		Name:       "Simple Light on Motion",
		Triggers: []model.Trigger{
			{
				ID:         "trig-cam",
				Kind:       model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "notify-step",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{ID: "notification.send", Version: "1.0.0"},
						Arguments: map[string]model.Expr{
							"title": model.Literal(titleVal),
						},
					},
				},
			},
		},
	}

	manifest, diags := comp.Compile(scen)
	if diags.HasErrors() {
		t.Fatalf("compilation failed: %v", diags.Error())
	}

	if manifest.SelectedRuntime != RuntimeAxiom {
		t.Fatalf("expected RuntimeAxiom, got %s (Reasons: %v)", manifest.SelectedRuntime, manifest.RuntimeReasons)
	}

	if manifest.AxiomPlan == nil {
		t.Fatalf("expected AxiomPlan to be populated")
	}
	if manifest.AxiomPlan.ActionCap != "notification.send" {
		t.Fatalf("expected ActionCap notification.send, got %s", manifest.AxiomPlan.ActionCap)
	}
	if manifest.PlanDigest == "" {
		t.Fatalf("expected non-empty PlanDigest")
	}
}

func TestCompiler_ADGOLowering_DoorUnlock_SafetyAugmentation(t *testing.T) {
	reg := setupTestRegistry(t)
	comp := NewCompiler(reg)

	scen := model.Scenario{
		ID:         "unlock-scenario",
		RevisionID: "rev-1",
		Name:       "Front Door Unlock",
		Triggers: []model.Trigger{
			{
				ID:         "manual-trigger",
				Kind:       model.TriggerManual,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "unlock-step",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{
							ID:      "door.unlock",
							Version: "1.0.0",
							Entity:  &model.EntityRef{ID: "front_door", Kind: "door"},
						},
					},
				},
			},
		},
	}

	manifest, diags := comp.Compile(scen)
	if diags.HasErrors() {
		t.Fatalf("compilation failed: %v", diags.Error())
	}

	if manifest.SelectedRuntime != RuntimeADGO {
		t.Fatalf("expected RuntimeADGO for unlock, got %s", manifest.SelectedRuntime)
	}

	// Verify Safety Augmentations
	if len(manifest.SafetyAugmentations) == 0 {
		t.Fatalf("expected safety augmentations for door unlock, got 0")
	}

	hasApproval := false
	hasLock := false
	hasVerify := false
	for _, aug := range manifest.SafetyAugmentations {
		if aug.Kind == "human_approval" {
			hasApproval = true
		}
		if aug.Kind == "resource_reservation" {
			hasLock = true
		}
		if aug.Kind == "verify_write" {
			hasVerify = true
		}
	}

	if !hasApproval || !hasLock || !hasVerify {
		t.Fatalf("missing required safety gates: approval=%v lock=%v verify=%v", hasApproval, hasLock, hasVerify)
	}

	// Verify User Graph vs System Graph separation
	if len(manifest.UserGraph.Nodes) >= len(manifest.SystemGraph.Nodes) {
		t.Fatalf("expected SystemGraph to contain more nodes than UserGraph due to safety gates (User: %d, System: %d)",
			len(manifest.UserGraph.Nodes), len(manifest.SystemGraph.Nodes))
	}
}

func TestCompiler_ADGOLowering_SirenCompensation(t *testing.T) {
	reg := setupTestRegistry(t)
	comp := NewCompiler(reg)

	durVal, _ := model.NewDurationValue(20 * time.Second)
	scen := model.Scenario{
		ID:         "siren-scenario",
		RevisionID: "rev-1",
		Name:       "Emergency Siren",
		Triggers: []model.Trigger{
			{
				ID:         "manual-trigger",
				Kind:       model.TriggerManual,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "siren-step",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{
							ID:      "siren.sound",
							Version: "1.0.0",
							Entity:  &model.EntityRef{ID: "yard_siren", Kind: "siren"},
						},
						Arguments: map[string]model.Expr{
							"duration": model.Literal(durVal),
						},
					},
				},
			},
		},
	}

	manifest, diags := comp.Compile(scen)
	if diags.HasErrors() {
		t.Fatalf("compilation failed: %v", diags.Error())
	}

	if manifest.SelectedRuntime != RuntimeADGO {
		t.Fatalf("expected RuntimeADGO for siren, got %s", manifest.SelectedRuntime)
	}

	hasCompensation := false
	for _, aug := range manifest.SafetyAugmentations {
		if aug.Kind == "compensation" {
			hasCompensation = true
			break
		}
	}
	if !hasCompensation {
		t.Fatalf("expected ensure-disabled compensation for siren action")
	}
}

func TestCompiler_DeterminismAndLayoutIndependence(t *testing.T) {
	reg := setupTestRegistry(t)
	comp := NewCompiler(reg)

	titleVal, _ := model.ValueOf("Alert")
	scen1 := model.Scenario{
		ID:         "det-scen",
		RevisionID: "rev-1",
		Name:       "Deterministic Test",
		Triggers: []model.Trigger{
			{
				ID:         "trig1",
				Kind:       model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "act1",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{ID: "notification.send", Version: "1.0.0"},
						Arguments:  map[string]model.Expr{"title": model.Literal(titleVal)},
					},
				},
			},
		},
	}

	scen2 := scen1
	scen2.Metadata.Layout = map[string]model.NodeLayout{
		"act1": {X: 450, Y: 600, Collapsed: true},
	}

	m1, diags1 := comp.Compile(scen1)
	if diags1.HasErrors() {
		t.Fatalf("comp 1 failed: %v", diags1.Error())
	}
	m2, diags2 := comp.Compile(scen2)
	if diags2.HasErrors() {
		t.Fatalf("comp 2 failed: %v", diags2.Error())
	}

	if m1.SemanticDigest != m2.SemanticDigest {
		t.Fatalf("semantic digest mismatch: %s vs %s", m1.SemanticDigest, m2.SemanticDigest)
	}
	if m1.PlanDigest != m2.PlanDigest {
		t.Fatalf("plan digest mismatch: %s vs %s", m1.PlanDigest, m2.PlanDigest)
	}
}

func TestCompiler_StaticConflictDetection(t *testing.T) {
	reg := setupTestRegistry(t)
	comp := NewCompiler(reg)

	// Self-recursion test
	selfRecScen := model.Scenario{
		ID:         "recursive-flow",
		RevisionID: "rev-1",
		Name:       "Recursive",
		Triggers: []model.Trigger{
			{
				ID:         "trig",
				Kind:       model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "sub-call",
					Kind: model.StepSubflow,
					Subflow: &model.SubflowStep{
						ScenarioID: "recursive-flow",
						Version:    1,
					},
				},
			},
		},
	}

	_, diags := comp.Compile(selfRecScen)
	foundSelfRec := false
	for _, d := range diags {
		if d.Code == CodeSelfRecursion {
			foundSelfRec = true
			break
		}
	}
	if !foundSelfRec {
		t.Fatalf("expected CodeSelfRecursion diagnostic for self-recursive scenario")
	}
}
