package incidents

import (
	"encoding/json"
	"fmt"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	"testing"
	"time"
)

func eid(prefix string, n int) domain.ID { return domain.ID(fmt.Sprintf("%s_%026d", prefix, n)) }
func ev(n int, typ string, at time.Time, payload any) events.Envelope {
	b, _ := json.Marshal(payload)
	return events.Envelope{SchemaVersion: 1, ID: eid("evt", n), Type: typ, Source: "test", OccurredAt: at, ReceivedAt: at, CorrelationID: eid("cor", n), Severity: events.SeverityInfo, Payload: b}
}
func TestCorrelationDoorbellJoinsRecentPersonButDistinctPeopleSeparate(t *testing.T) {
	base := time.Unix(1000, 0)
	in := []events.Envelope{ev(1, "frigate.review.updated", base, map[string]any{"camera_id": "front", "event_id": "person-a"}), ev(2, "intercom.button.pressed", base.Add(3*time.Second), map[string]any{"device_id": "front", "location": "front"}), ev(3, "frigate.review.updated", base.Add(5*time.Second), map[string]any{"camera_id": "front", "event_id": "person-b"})}
	next := 0
	idf := func(string) (domain.ID, error) { next++; return eid("inc", next), nil }
	out, err := Correlate(in, 45*time.Second, idf)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("incidents=%+v", out)
	}
	if len(out[0].EventIDs) != 2 {
		t.Fatalf("doorbell not joined: %+v", out[0])
	}
}
