package correlation

import (
	"sync"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/domain/observation"
)

func testObservation(id, subject, source, kind string, at time.Time) observation.Observation {
	return observation.Observation{
		EventID: id, SubjectKey: subject, SourceID: source, Kind: kind,
		OccurredAt: at, ReceivedAt: at.Add(100 * time.Millisecond), Confidence: 0.9,
	}
}

func TestDuplicateAndOutOfOrderWithinLateness(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	c := MustNew(DefaultConfig())
	first, err := c.Ingest(testObservation("e1", "person:42", "cam-a", "person", base.Add(2*time.Second)))
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Status != StatusNewGroup || first.Candidate.EventCount != 1 {
		t.Fatalf("unexpected first result: %#v", first)
	}

	older, err := c.Ingest(testObservation("e0", "person:42", "cam-b", "person", base))
	if err != nil {
		t.Fatalf("out-of-order ingest: %v", err)
	}
	if older.Status != StatusUpdated || older.Candidate.EventCount != 2 {
		t.Fatalf("out-of-order event not merged: %#v", older)
	}
	if older.Candidate.EventIDs[0] != "e0" || older.Candidate.SourceCount != 2 {
		t.Fatalf("events not sorted/correlated: %#v", older.Candidate)
	}

	duplicate, err := c.Ingest(testObservation("e1", "person:42", "cam-a", "person", base.Add(2*time.Second)))
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if duplicate.Status != StatusDuplicate || duplicate.Candidate.EventCount != 2 {
		t.Fatalf("duplicate changed group: %#v", duplicate)
	}
}

func TestLateEventIsRejectedWithoutMutatingGroup(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	cfg := DefaultConfig()
	cfg.MaxLateness = 2 * time.Second
	c := MustNew(cfg)
	_, _ = c.Ingest(testObservation("new", "subject", "cam-a", "person", base.Add(10*time.Second)))
	late, err := c.Ingest(testObservation("late", "subject", "cam-b", "person", base))
	if err != nil {
		t.Fatalf("late ingest: %v", err)
	}
	if late.Status != StatusLate || late.Candidate.EventCount != 1 {
		t.Fatalf("late event mutated group: %#v", late)
	}
}

func TestCrossCameraAggregationBuildsSecurityContext(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	c := MustNew(DefaultConfig())
	one := testObservation("a", "person:x", "front", "vision.person.detected.v1", base)
	one.Context = incident.SecurityContext{Identity: incident.IdentityKnown, AlarmMode: "away"}
	two := testObservation("b", "person:x", "hall", "vision.person.detected.v1", base.Add(4*time.Second))
	two.Context = incident.SecurityContext{Identity: incident.IdentityUnknown, EntryActive: true}
	_, _ = c.Ingest(one)
	result, err := c.Ingest(two)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	trigger := result.Candidate.Trigger
	if trigger.Kind != "correlated.person.activity.v1" || trigger.Context.Identity != incident.IdentityUnknown {
		t.Fatalf("unexpected correlated trigger: %#v", trigger)
	}
	if !trigger.Context.EntryActive || trigger.Context.CrossCameraMatches != 1 || trigger.Context.DwellSeconds != 4 {
		t.Fatalf("context aggregation failed: %#v", trigger.Context)
	}
}

func TestGapStartsNewStableGroup(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	cfg := DefaultConfig()
	cfg.Window = 5 * time.Second
	cfg.MaxLateness = time.Second
	c := MustNew(cfg)
	first, _ := c.Ingest(testObservation("a", "subject", "cam", "motion", base))
	second, _ := c.Ingest(testObservation("b", "subject", "cam", "motion", base.Add(6*time.Second)))
	if second.Status != StatusNewGroup || first.Candidate.CorrelationID == second.Candidate.CorrelationID {
		t.Fatalf("new window did not create new group: %#v %#v", first, second)
	}
}

func TestGroupAndSeenMemoryAreBounded(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	cfg := DefaultConfig()
	cfg.MaxEventsPerGroup = 3
	cfg.MaxSeenEvents = 4
	c := MustNew(cfg)
	var last Result
	for i := 0; i < 8; i++ {
		id := string(rune('a' + i))
		last, _ = c.Ingest(testObservation(id, "subject", "cam", "motion", base.Add(time.Duration(i)*time.Second)))
	}
	if last.Candidate.EventCount != 3 {
		t.Fatalf("event group not bounded: %d", last.Candidate.EventCount)
	}
	if len(c.seen) > cfg.MaxSeenEvents {
		t.Fatalf("seen set not bounded: %d", len(c.seen))
	}
}

func TestConcurrentIngestIsSafe(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	c := MustNew(DefaultConfig())
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := string(rune(0x1000 + i))
			_, _ = c.Ingest(testObservation(id, "subject", "cam", "motion", base.Add(time.Duration(i)*time.Millisecond)))
		}()
	}
	wg.Wait()
}
