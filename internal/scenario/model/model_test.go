package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestScenarioJSONRoundTrip(t *testing.T) {
	source := sampleScenario(t)
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeScenario(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(source, decoded) {
		t.Fatalf("round-trip mismatch:\nwant %#v\n got %#v", source, decoded)
	}
}

func TestNormalizeAndSemanticDigestAreDeterministic(t *testing.T) {
	first := sampleScenario(t)
	second := sampleScenario(t)
	second.Name = "  Entry alert  "
	second.Metadata.Tags = []string{"security", "night", "security"}
	second.Triggers[0], second.Triggers[1] = second.Triggers[1], second.Triggers[0]

	firstJSON, err := CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := CanonicalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("canonical JSON differs:\n%s\n%s", firstJSON, secondJSON)
	}

	firstDigest, err := SemanticDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := SemanticDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("semantic digest differs: %s != %s", firstDigest, secondDigest)
	}
}

func TestLayoutDoesNotAffectSemanticDigestButBehaviorDoes(t *testing.T) {
	scenario := sampleScenario(t)
	baseline, err := SemanticDigest(scenario)
	if err != nil {
		t.Fatal(err)
	}

	layoutOnly := sampleScenario(t)
	layoutOnly.Metadata.Layout["notify"] = NodeLayout{X: 900, Y: 120, Collapsed: true}
	layoutDigest, err := SemanticDigest(layoutOnly)
	if err != nil {
		t.Fatal(err)
	}
	if baseline != layoutDigest {
		t.Fatal("UI layout changed semantic digest")
	}

	changed := sampleScenario(t)
	changed.Flow.Steps[1].Wait.Duration = 45 * time.Second
	changedDigest, err := SemanticDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseline == changedDigest {
		t.Fatal("semantic change did not change digest")
	}
}

func TestCloneCreatesNewDraftIdentity(t *testing.T) {
	source := sampleScenario(t)
	source.Version = 7
	source.Enabled = true
	clone, err := Clone(source)
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID == source.ID || clone.RevisionID == source.RevisionID {
		t.Fatal("clone reused source identity")
	}
	if clone.Version != 0 || clone.Enabled {
		t.Fatal("clone must be a disabled draft")
	}
}

func TestUnknownStepKindFailsClosed(t *testing.T) {
	scenario := sampleScenario(t)
	scenario.Flow.Steps[0].Kind = StepKind("teleport")
	raw, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeScenario(raw); err == nil {
		t.Fatal("unknown step kind was accepted")
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	scenario := sampleScenario(t)
	raw, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["providerPayload"] = map[string]any{"unsafe": true}
	raw, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeScenario(raw); err == nil {
		t.Fatal("unknown top-level field was accepted")
	}
}

func TestDuplicateNestedStepIDFails(t *testing.T) {
	scenario := sampleScenario(t)
	literal, err := ValueOf(true)
	if err != nil {
		t.Fatal(err)
	}
	scenario.Flow.Steps = append(scenario.Flow.Steps, Step{
		ID:   "branch",
		Kind: StepIf,
		If: &IfStep{
			Condition: Expr{Op: "literal", Value: &literal},
			Then:      Flow{Steps: []Step{{ID: "notify", Kind: StepStop, Stop: &StopStep{Outcome: StopCompleted}}}},
		},
	})
	if err := scenario.Validate(); err == nil {
		t.Fatal("duplicate nested step id was accepted")
	}
}

func sampleScenario(t *testing.T) Scenario {
	t.Helper()
	confidence, err := ValueOf(0.85)
	if err != nil {
		t.Fatal(err)
	}
	away, err := ValueOf("away")
	if err != nil {
		t.Fatal(err)
	}
	message, err := ValueOf("Unknown person at entrance")
	if err != nil {
		t.Fatal(err)
	}
	return Scenario{
		ID:          "scenario-entry-alert",
		RevisionID:  "rev-entry-alert",
		Version:     0,
		Name:        "Entry alert",
		Description: "Notify when an unknown person appears while away.",
		Triggers: []Trigger{
			{
				ID:         "trigger-person",
				Kind:       TriggerDeviceEvent,
				Capability: CapabilityRef{ID: "camera.person.detected", Version: "1.0.0", Entity: &EntityRef{ID: "front-camera", Kind: "camera"}},
				Parameters: map[string]Value{"min_confidence": confidence},
			},
			{
				ID:         "trigger-manual",
				Kind:       TriggerManual,
				Capability: CapabilityRef{ID: "core.manual", Version: "1.0.0"},
			},
		},
		Condition: Expr{Op: "eq", Args: []Expr{
			{Op: "ref", Ref: "home.mode"},
			{Op: "literal", Value: &away},
		}},
		Flow: Flow{Steps: []Step{
			{
				ID:   "notify",
				Kind: StepAction,
				Action: &ActionStep{
					Capability: CapabilityRef{ID: "notification.send", Version: "1.0.0"},
					Arguments:  map[string]Expr{"message": Literal(message)},
				},
			},
			{ID: "wait", Kind: StepWait, Wait: &WaitStep{Duration: 30 * time.Second}},
			{ID: "done", Kind: StepStop, Stop: &StopStep{Outcome: StopCompleted}},
		}},
		Metadata: Metadata{
			Tags: []string{"night", "security"},
			Layout: map[string]NodeLayout{
				"notify": {X: 100, Y: 200},
				"wait":   {X: 300, Y: 200},
				"done":   {X: 500, Y: 200},
			},
		},
	}
}
