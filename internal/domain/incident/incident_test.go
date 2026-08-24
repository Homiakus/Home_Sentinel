package incident

import (
	"testing"
	"time"
)

func TestExecutionIDDeterministic(t *testing.T) {
	trigger := Trigger{EventID: "evt-9", SourceID: "front", Kind: "person", OccurredAt: time.Now().UTC(), Confidence: 0.9}
	first := ExecutionID(trigger)
	second := ExecutionID(trigger)
	if first != second {
		t.Fatalf("execution id is not deterministic: %q != %q", first, second)
	}
	other := trigger
	other.EventID = "evt-10"
	if first == ExecutionID(other) {
		t.Fatal("different trigger produced same execution id")
	}
}

func TestTriggerRejectsInvalidConfidence(t *testing.T) {
	trigger := Trigger{EventID: "evt", SourceID: "front", Kind: "person", OccurredAt: time.Now().UTC(), Confidence: 1.1}
	if err := trigger.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
