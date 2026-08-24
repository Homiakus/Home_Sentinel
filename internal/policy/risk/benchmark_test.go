package risk

import (
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
)

func BenchmarkAssess(b *testing.B) {
	policy := DefaultPolicy()
	features := Features{
		DetectionConfidence: 0.95, EvidenceCount: 3, PersonDetected: true,
		Identity: incident.IdentityUnknown, AlarmMode: "away", EntryActive: true,
		DwellSeconds: 90, CrossCameraMatches: 2,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := policy.Assess(features); err != nil {
			b.Fatal(err)
		}
	}
}
