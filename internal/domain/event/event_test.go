package event

import (
	"testing"
	"time"
)

func TestEnvelopeValidate(t *testing.T) {
	now := time.Now().UTC()
	valid := Envelope{
		ID: "evt-1", Kind: "vision.person.detected.v1",
		Source: Source{ID: "front", Type: "camera"},
		OccurredAt: now, ReceivedAt: now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}

	missingID := valid
	missingID.ID = ""
	if err := missingID.Validate(); err == nil {
		t.Fatal("expected missing id error")
	}

	future := valid
	future.OccurredAt = now.Add(MaxClockSkew + time.Second)
	if err := future.Validate(); err == nil {
		t.Fatal("expected clock skew error")
	}
}
