//go:build sqlite_cgo

package incidents

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

func TestServicePersistsCorrelatedIncident(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "db.sqlite"), BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	s := NewService(bus, repository.NewStore[events.Envelope](db, repository.KindEvent), repository.NewStore[Incident](db, repository.KindIncident))
	base := time.Now().UTC()
	mk := func(id, typ string, at time.Time, payload any) events.Envelope {
		b, _ := json.Marshal(payload)
		return events.Envelope{SchemaVersion: 1, ID: domain.ID(id), Type: typ, Source: "test", OccurredAt: at, ReceivedAt: at, CorrelationID: domain.ID("cor_00000000000000000000000001"), Severity: events.SeverityInfo, Payload: b}
	}
	person := mk("evt_00000000000000000000000001", "frigate.review.updated", base, map[string]any{"camera_id": "front", "event_id": "person-1"})
	bell := mk("evt_00000000000000000000000002", "intercom.button.pressed", base.Add(time.Second), map[string]any{"device_id": "door", "location": "front"})
	if err := s.Ingest(ctx, person); err != nil {
		t.Fatal(err)
	}
	if err := s.Ingest(ctx, bell); err != nil {
		t.Fatal(err)
	}
	items, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].Value.EventIDs) != 2 {
		t.Fatalf("incidents=%+v", items)
	}
	if _, err := s.Event(ctx, person.ID.String()); err != nil {
		t.Fatal(err)
	}
}
