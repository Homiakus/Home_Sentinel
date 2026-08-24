package correlation

import (
	"strconv"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/observation"
)

func BenchmarkCorrelatorIngest(b *testing.B) {
	config := DefaultConfig()
	config.Window = time.Hour
	config.MaxEventsPerGroup = 64
	config.MaxSeenEvents = 8192
	correlator := MustNew(config)
	base := time.Unix(1_700_000_000, 0).UTC()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		at := base.Add(time.Duration(i) * time.Millisecond)
		_, err := correlator.Ingest(observation.Observation{
			EventID: "bench-" + strconv.Itoa(i), SubjectKey: "person:benchmark",
			SourceID: "front", Kind: "vision.person.detected.v1",
			OccurredAt: at, ReceivedAt: at, Confidence: 0.9,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
