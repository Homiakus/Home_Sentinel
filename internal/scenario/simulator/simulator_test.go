package simulator

import (
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

var allowedHomeModes = []string{"armed_away", "armed_home", "armed_night", "disarmed", "vacation"}

func setupSimulator(t *testing.T) *Simulator {
	reg := capability.NewRegistry(nil)

	notifDesc, _ := capability.NewActionDescriptor("notification.send", "1.0.0", "core", "notify", "alert", "Send Notification", "notification:send")
	notifDesc.Input = capability.Schema{
		Fields: []capability.FieldSchema{
			{Name: "title", Type: model.TypeRef{Kind: model.TypeString}, Required: true},
		},
	}
	if err := reg.Register(notifDesc); err != nil {
		t.Fatalf("register notif: %v", err)
	}

	sirenDesc, _ := capability.NewActionDescriptor("siren.sound", "1.0.0", "core", "alarm", "siren", "Sound Siren", "siren:sound")
	sirenDesc.Risk = model.RiskHigh
	sirenDesc.ExternalEffect = true
	sirenDesc.Idempotency = capability.IdempotencySupported
	sirenDesc.EntityKinds = []string{"siren"}
	if err := reg.Register(sirenDesc); err != nil {
		t.Fatalf("register siren: %v", err)
	}

	doorDesc, _ := capability.NewActionDescriptor("door.unlock", "1.0.0", "core", "access", "door", "Unlock Door", "door:unlock")
	doorDesc.Risk = model.RiskCritical
	doorDesc.ExternalEffect = true
	doorDesc.Idempotency = capability.IdempotencyRequired
	doorDesc.EntityKinds = []string{"door"}
	if err := reg.Register(doorDesc); err != nil {
		t.Fatalf("register door: %v", err)
	}

	camDesc, _ := capability.NewTriggerDescriptor("camera.person.detected", "1.0.0", "frigate", "nvr", "vision", "Person Detected", "camera:read")
	if err := reg.Register(camDesc); err != nil {
		t.Fatalf("register cam: %v", err)
	}

	comp := compiler.NewCompiler(reg)
	return NewSimulator(comp)
}

func TestSimulator_PureSimulation_WouldExecuteOnly(t *testing.T) {
	sim := setupSimulator(t)

	titleVal, _ := model.ValueOf("Perimeter Alert")
	awayModeVal, _ := model.NewEnumValue("home_mode", allowedHomeModes, "armed_away")

	scen := model.Scenario{
		ID:         "night-perimeter-alert",
		RevisionID: "rev-1",
		Name:       "Night Perimeter Alert",
		Triggers: []model.Trigger{
			{
				ID:         "cam-person",
				Kind:       model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Condition: model.Expr{
			Op: "eq",
			Args: []model.Expr{
				model.Ref("home.mode"),
				model.Literal(awayModeVal),
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "step-notify",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{ID: "notification.send", Version: "1.0.0"},
						Arguments:  map[string]model.Expr{"title": model.Literal(titleVal)},
					},
				},
				{
					ID:   "step-wait",
					Kind: model.StepWait,
					Wait: &model.WaitStep{Duration: 30 * time.Second},
				},
				{
					ID:   "step-siren",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{
							ID:      "siren.sound",
							Version: "1.0.0",
							Entity:  &model.EntityRef{ID: "backyard_siren", Kind: "siren"},
						},
					},
				},
			},
		},
	}

	clock := NewVirtualClock(time.Date(2026, 8, 25, 23, 0, 0, 0, time.UTC))

	ctx := SimulationContext{
		Mode: ModePure,
		HomeState: map[string]model.Value{
			"home.mode": awayModeVal,
		},
		TriggerEvent: map[string]model.Value{
			"person_name": mustVal("unknown_visitor"),
		},
	}

	res, err := sim.Simulate(scen, ctx, clock)
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}

	if !res.Passed {
		t.Fatalf("Simulation did not pass: %v", res.Errors)
	}

	if res.FinalOutcome != "COMPLETED" {
		t.Fatalf("expected final outcome COMPLETED, got %s", res.FinalOutcome)
	}

	// Verify virtual clock advanced by 30 seconds
	if res.SimulatedDuration != 30*time.Second {
		t.Fatalf("expected simulated duration 30s, got %v", res.SimulatedDuration)
	}

	// Verify hypothetical effects (MUST only be WOULD_EXECUTE)
	if len(res.HypotheticalEffects) != 2 {
		t.Fatalf("expected 2 hypothetical effects, got %d", len(res.HypotheticalEffects))
	}
	for _, eff := range res.HypotheticalEffects {
		if eff.Action != "WOULD_EXECUTE" {
			t.Fatalf("expected action WOULD_EXECUTE, got %s", eff.Action)
		}
	}

	// Verify explanation traces
	if len(res.Traces) == 0 {
		t.Fatalf("expected non-empty trace steps")
	}
	hasTriggerTrace := false
	hasConditionTrace := false
	hasWaitTrace := false
	for _, tr := range res.Traces {
		if tr.Kind == "trigger" && tr.Outcome == "MATCH" {
			hasTriggerTrace = true
		}
		if tr.Kind == "condition" && tr.Outcome == "CONDITION_TRUE" {
			hasConditionTrace = true
		}
		if tr.Kind == "wait" && tr.Outcome == "WAITED" {
			hasWaitTrace = true
		}
	}
	if !hasTriggerTrace || !hasConditionTrace || !hasWaitTrace {
		t.Fatalf("missing expected traces: trig=%v cond=%v wait=%v", hasTriggerTrace, hasConditionTrace, hasWaitTrace)
	}
}

func TestSimulator_ConditionFalse_SkipsExecution(t *testing.T) {
	sim := setupSimulator(t)

	awayModeVal, _ := model.NewEnumValue("home_mode", allowedHomeModes, "armed_away")
	disarmedModeVal, _ := model.NewEnumValue("home_mode", allowedHomeModes, "disarmed")

	scen := model.Scenario{
		ID:         "skip-scen",
		RevisionID: "rev-1",
		Name:       "Skip on Disarmed",
		Triggers: []model.Trigger{
			{
				ID:         "cam-trig",
				Kind:       model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Condition: model.Expr{
			Op: "eq",
			Args: []model.Expr{
				model.Ref("home.mode"),
				model.Literal(awayModeVal),
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "notify-step",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{ID: "notification.send", Version: "1.0.0"},
						Arguments:  map[string]model.Expr{"title": model.Literal(mustVal("Hello"))},
					},
				},
			},
		},
	}

	ctx := SimulationContext{
		Mode: ModePure,
		HomeState: map[string]model.Value{
			"home.mode": disarmedModeVal, // Condition evaluates to FALSE
		},
	}

	res, err := sim.Simulate(scen, ctx, nil)
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}

	if res.FinalOutcome != "CONDITION_SKIPPED" {
		t.Fatalf("expected CONDITION_SKIPPED, got %s", res.FinalOutcome)
	}
	if len(res.HypotheticalEffects) != 0 {
		t.Fatalf("expected 0 hypothetical effects when condition skipped, got %d", len(res.HypotheticalEffects))
	}
}

func mustVal(v any) model.Value {
	val, _ := model.ValueOf(v)
	return val
}
