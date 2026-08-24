package risk

import (
	"reflect"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
)

func TestAssessmentIsDeterministicAndExplainable(t *testing.T) {
	features := Features{
		DetectionConfidence: 0.95,
		EvidenceCount:       2,
		PersonDetected:      true,
		Identity:            incident.IdentityUnknown,
		AlarmMode:           "away",
		EntryActive:         true,
		DwellSeconds:        90,
		CrossCameraMatches:  2,
	}
	policy := DefaultPolicy()
	first, err := policy.Assess(features)
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	second, err := policy.Assess(features)
	if err != nil {
		t.Fatalf("second assess: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("assessment is not deterministic:\n%#v\n%#v", first, second)
	}
	if first.Risk != incident.RiskCritical {
		t.Fatalf("expected critical risk, got %s score=%f", first.Risk, first.Score)
	}
	if len(first.Contributions) < 6 || len(first.Reasons) < 5 {
		t.Fatalf("assessment lacks explanation: %#v", first)
	}
}

func TestThresholdBoundaries(t *testing.T) {
	policy := Policy{MediumThreshold: 0.30, HighThreshold: 0.60, CriticalThreshold: 0.90}
	cases := []struct {
		confidence float64
		person     bool
		want       incident.Risk
	}{
		{confidence: 0.5, want: incident.RiskLow},
		{confidence: 1.0, want: incident.RiskMedium},
		{confidence: 1.0, person: true, want: incident.RiskMedium},
	}
	for _, tc := range cases {
		got, err := policy.Assess(Features{DetectionConfidence: tc.confidence, PersonDetected: tc.person})
		if err != nil {
			t.Fatalf("assess: %v", err)
		}
		if got.Risk != tc.want {
			t.Fatalf("confidence=%f person=%v: want %s got %s score=%f", tc.confidence, tc.person, tc.want, got.Risk, got.Score)
		}
	}
}

func TestInvalidNumericInputRejected(t *testing.T) {
	if _, err := DefaultPolicy().Assess(Features{DetectionConfidence: 1.01}); err == nil {
		t.Fatal("expected confidence validation error")
	}
	if _, err := DefaultPolicy().Assess(Features{DetectionConfidence: 0.5, EvidenceCount: -1}); err == nil {
		t.Fatal("expected negative count validation error")
	}
}
