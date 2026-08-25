package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

func TestEnvelopeValidate(t *testing.T) {
	eid, _ := domain.NewID("evt")
	cid, _ := domain.NewID("cor")
	now := time.Now().UTC()
	e := Envelope{SchemaVersion: SchemaV1, ID: eid, Type: "system.started", Source: "sentinel", OccurredAt: now, ReceivedAt: now, CorrelationID: cid, Severity: SeverityInfo, Payload: json.RawMessage(`{"ok":true}`)}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	e.SchemaVersion = 999
	if err := e.Validate(); err == nil {
		t.Fatal("expected unsupported schema error")
	}
}
