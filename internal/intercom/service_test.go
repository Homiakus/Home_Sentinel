//go:build sqlite_cgo

package intercom

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
	"github.com/Homiakus/Home_Sentinel/internal/locks"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type fakeMQTT struct {
	messages []mqttint.Message
	err      error
}

func (f *fakeMQTT) Publish(_ context.Context, m mqttint.Message) error {
	f.messages = append(f.messages, m)
	return f.err
}

func newTestService(t *testing.T) (*Service, *fakeMQTT, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	m := &fakeMQTT{}
	s := &Service{Devices: repository.NewStore[Device](db, repository.KindIntercom), States: repository.NewStore[ObservedState](db, repository.KindIntercomState), Commands: CommandStore{DB: db}, MQTT: m, Events: events.NewBus(), Locks: locks.New(), Now: func() time.Time { return time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC) }}
	t.Cleanup(func() { s.Events.Close() })
	return s, m, ctx
}

func TestUnlockCommandIsExpiringNonRetainedAndReplaySafe(t *testing.T) {
	s, m, ctx := newTestService(t)
	dev := Device{ID: "front_door", Name: "Front door", Capabilities: Capabilities{Lock: true, Button: true, DoorSensor: true}}
	if _, err := s.Put(ctx, dev); err != nil {
		t.Fatal(err)
	}
	corr, _ := domain.NewID("cor")
	rec, err := s.Unlock(ctx, UnlockRequest{DeviceID: dev.ID, ActorID: "usr_admin", CorrelationID: corr.String(), TTL: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.messages) != 1 {
		t.Fatalf("messages=%d", len(m.messages))
	}
	msg := m.messages[0]
	if msg.Retained {
		t.Fatal("unlock command must never be retained")
	}
	if msg.Topic != "sentinel/intercom/front_door/command/unlock" {
		t.Fatalf("topic=%s", msg.Topic)
	}
	var cmd Command
	if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
		t.Fatal(err)
	}
	if cmd.RequestID != rec.RequestID || cmd.ExpiresAt.Sub(cmd.IssuedAt) != 5*time.Second {
		t.Fatalf("cmd=%+v", cmd)
	}
	ack, _ := json.Marshal(Ack{SchemaVersion: 1, RequestID: rec.RequestID, Accepted: true})
	if err := s.IngestMQTT(ctx, mqttint.Message{Topic: "sentinel/intercom/front_door/event/ack", Payload: ack}); err != nil {
		t.Fatal(err)
	}
	if err := s.IngestMQTT(ctx, mqttint.Message{Topic: "sentinel/intercom/front_door/event/ack", Payload: ack}); err == nil {
		t.Fatal("replayed ack accepted")
	}
	stored, err := s.Commands.Get(ctx, rec.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "acknowledged" {
		t.Fatalf("status=%s", stored.Status)
	}
}

func TestIndependentStateSequencesAndButtonDedup(t *testing.T) {
	s, _, ctx := newTestService(t)
	if _, err := s.Put(ctx, Device{ID: "entry", Name: "Entry", Capabilities: Capabilities{Button: true, DoorSensor: true, Lock: true}}); err != nil {
		t.Fatal(err)
	}
	at := s.now()
	if err := s.IngestMQTT(ctx, mqttint.Message{Topic: "sentinel/intercom/entry/state/availability", Payload: EncodeState("online", 10, at)}); err != nil {
		t.Fatal(err)
	}
	if err := s.IngestMQTT(ctx, mqttint.Message{Topic: "sentinel/intercom/entry/state/door", Payload: EncodeState("open", 1, at)}); err != nil {
		t.Fatal(err)
	}
	if err := s.IngestMQTT(ctx, mqttint.Message{Topic: "sentinel/intercom/entry/state/lock", Payload: EncodeState("locked", 2, at)}); err != nil {
		t.Fatal(err)
	}
	st, err := s.State(ctx, "entry")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Available || st.Door != DoorOpen || st.Lock != LockLocked {
		t.Fatalf("state=%+v", st)
	}
	if st.Sequences["availability"] != 10 || st.Sequences["door"] != 1 || st.Sequences["lock"] != 2 {
		t.Fatalf("seq=%v", st.Sequences)
	}
	sub := s.Events.Subscribe(4)
	defer sub.Cancel()
	ch := sub.C
	payload, _ := json.Marshal(map[string]any{"schema_version": 1, "pressed": true, "sequence": 7, "occurred_at": at, "location": "entrance"})
	msg := mqttint.Message{Topic: "sentinel/intercom/entry/event/button", Payload: payload}
	if err := s.IngestMQTT(ctx, msg); err != nil {
		t.Fatal(err)
	}
	if err := s.IngestMQTT(ctx, msg); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("missing button event")
	}
	select {
	case <-ch:
		t.Fatal("duplicate button event")
	case <-time.After(25 * time.Millisecond):
	}
}

func TestExpiredAckRejected(t *testing.T) {
	s, _, ctx := newTestService(t)
	if _, err := s.Put(ctx, Device{ID: "front", Name: "Front", Capabilities: Capabilities{Lock: true}}); err != nil {
		t.Fatal(err)
	}
	corr, _ := domain.NewID("cor")
	rec, err := s.Unlock(ctx, UnlockRequest{DeviceID: "front", ActorID: "usr_admin", CorrelationID: corr.String(), TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return time.Date(2026, 8, 16, 20, 0, 2, 0, time.UTC) }
	ack, _ := json.Marshal(Ack{SchemaVersion: 1, RequestID: rec.RequestID, Accepted: true})
	if err := s.IngestMQTT(ctx, mqttint.Message{Topic: "sentinel/intercom/front/event/ack", Payload: ack}); err == nil {
		t.Fatal("expired ack accepted")
	}
}
