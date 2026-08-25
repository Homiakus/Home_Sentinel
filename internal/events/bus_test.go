package events

import (
	"encoding/json"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"testing"
	"time"
)

func validEnvelope(t *testing.T) Envelope {
	e, _ := domain.NewID("evt")
	c, _ := domain.NewID("cor")
	return Envelope{SchemaVersion: 1, ID: e, Type: "x", Source: "test", OccurredAt: time.Now(), ReceivedAt: time.Now(), CorrelationID: c, Severity: SeverityInfo, Payload: json.RawMessage(`{}`)}
}
func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	b := NewBus()
	s := b.Subscribe(1)
	defer s.Cancel()
	e := validEnvelope(t)
	if d := b.Publish(e); d != 0 {
		t.Fatal(d)
	}
	if d := b.Publish(e); d != 1 {
		t.Fatalf("drop=%d", d)
	}
	if s.Dropped() != 1 {
		t.Fatal("drop metric")
	}
}
