package model

import (
	"testing"
	"time"
)

func TestStage30B_ExpressionBindings(t *testing.T) {
	strVal, err := ValueOf("Front Porch")
	if err != nil {
		t.Fatalf("ValueOf string failed: %v", err)
	}
	intVal, err := ValueOf(int64(42))
	if err != nil {
		t.Fatalf("ValueOf int failed: %v", err)
	}
	durVal, err := NewDurationValue(10 * time.Second)
	if err != nil {
		t.Fatalf("NewDurationValue failed: %v", err)
	}

	env := TypeEnv{
		"trigger.person.name": TypeRef{Kind: TypeString},
		"trigger.snapshot":    TypeRef{Kind: TypeArtifactRef},
		"parameter.timeout":   TypeRef{Kind: TypeDuration, Unit: "ns"},
		"state.home.mode":     TypeRef{Kind: TypeString},
		"device.temp_c":       TypeRef{Kind: TypeTemperature, Unit: "celsius"},
	}

	// 1. Literal argument
	litExpr := Literal(strVal)
	typ, err := CheckExpr(litExpr, env)
	if err != nil {
		t.Fatalf("CheckExpr literal failed: %v", err)
	}
	if typ.Kind != TypeString {
		t.Fatalf("expected TypeString, got %s", typ.Kind)
	}

	// 2. Trigger reference
	trigRefExpr := Ref("trigger.person.name")
	typ, err = CheckExpr(trigRefExpr, env)
	if err != nil {
		t.Fatalf("CheckExpr trigger ref failed: %v", err)
	}
	if typ.Kind != TypeString {
		t.Fatalf("expected TypeString, got %s", typ.Kind)
	}

	// 3. Parameter reference
	paramRefExpr := Ref("parameter.timeout")
	typ, err = CheckExpr(paramRefExpr, env)
	if err != nil {
		t.Fatalf("CheckExpr param ref failed: %v", err)
	}
	if typ.Kind != TypeDuration {
		t.Fatalf("expected TypeDuration, got %s", typ.Kind)
	}

	// 4. Nested arithmetic expression
	arithExpr := Expr{
		Op: "add",
		Args: []Expr{
			Literal(intVal),
			Literal(intVal),
		},
	}
	typ, err = CheckExpr(arithExpr, env)
	if err != nil {
		t.Fatalf("CheckExpr arithmetic failed: %v", err)
	}
	if typ.Kind != TypeInt {
		t.Fatalf("expected TypeInt, got %s", typ.Kind)
	}

	// 5. Invalid reference fail closed
	badRefExpr := Ref("unknown.field.name")
	if _, err := CheckExpr(badRefExpr, env); err == nil {
		t.Fatalf("expected error for unknown reference, got nil")
	}

	// 6. ActionStep with typed Expr arguments in a Scenario
	scen := Scenario{
		ID:         "entry-alert",
		RevisionID: "rev-1",
		Name:       "Entry Alert",
		Triggers: []Trigger{
			{
				ID:         "person-detected",
				Kind:       TriggerDeviceEvent,
				Capability: CapabilityRef{ID: "camera.person.detected", Version: "1.0.0"},
			},
		},
		Parameters: []Parameter{
			{
				ID:      "timeout",
				Type:    TypeRef{Kind: TypeDuration, Unit: "ns"},
				Default: &durVal,
			},
		},
		Condition: Expr{
			Op: "eq",
			Args: []Expr{
				Ref("state.home.mode"),
				Literal(strVal),
			},
		},
		Flow: Flow{
			Steps: []Step{
				{
					ID:   "step-notify",
					Kind: StepAction,
					Action: &ActionStep{
						Capability: CapabilityRef{ID: "notify.user", Version: "1.0.0"},
						Arguments: map[string]Expr{
							"title":   Literal(strVal),
							"person":  Ref("trigger.person.name"),
							"timeout": Ref("parameter.timeout"),
						},
					},
				},
				{
					ID:   "step-subflow",
					Kind: StepSubflow,
					Subflow: &SubflowStep{
						ScenarioID: "subflow-security",
						Version:    1,
						Arguments: map[string]Expr{
							"mode": Ref("state.home.mode"),
						},
					},
				},
			},
		},
	}

	// Validation and Type Checking
	if err := scen.Validate(); err != nil {
		t.Fatalf("scen.Validate failed: %v", err)
	}

	baseEnv := TypeEnv{
		"trigger.person.name": TypeRef{Kind: TypeString},
		"state.home.mode":     TypeRef{Kind: TypeString},
	}
	if err := ValidateTypes(scen, baseEnv); err != nil {
		t.Fatalf("ValidateTypes failed: %v", err)
	}

	// Deterministic normalization & digest
	norm1, err := Normalize(scen)
	if err != nil {
		t.Fatalf("Normalize 1 failed: %v", err)
	}
	norm2, err := Normalize(scen)
	if err != nil {
		t.Fatalf("Normalize 2 failed: %v", err)
	}
	digest1, err := SemanticDigest(norm1)
	if err != nil {
		t.Fatalf("SemanticDigest 1 failed: %v", err)
	}
	digest2, err := SemanticDigest(norm2)
	if err != nil {
		t.Fatalf("SemanticDigest 2 failed: %v", err)
	}
	if digest1 != digest2 {
		t.Fatalf("expected identical digest, got %s vs %s", digest1, digest2)
	}

	// Layout does not change digest
	scenWithLayout := scen
	scenWithLayout.Metadata.Layout = map[string]NodeLayout{
		"step-notify": {X: 100, Y: 200},
	}
	digestWithLayout, err := SemanticDigest(scenWithLayout)
	if err != nil {
		t.Fatalf("SemanticDigest with layout failed: %v", err)
	}
	if digest1 != digestWithLayout {
		t.Fatalf("layout changed semantic digest: %s vs %s", digest1, digestWithLayout)
	}

	// Semantic expression change changes digest
	scenChanged := scen
	scenChanged.Flow.Steps[0].Action.Arguments["title"] = Ref("trigger.person.name")
	digestChanged, err := SemanticDigest(scenChanged)
	if err != nil {
		t.Fatalf("SemanticDigest changed failed: %v", err)
	}
	if digest1 == digestChanged {
		t.Fatalf("semantic expression change did not change digest")
	}
}
