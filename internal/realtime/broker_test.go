package realtime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBroker(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(1)
	defer cancel()
	b.Publish(Message{ID: "evt_1", Type: "test", Data: json.RawMessage(`{"x":1}`)})
	select {
	case m := <-ch:
		if m.ID != "evt_1" {
			t.Fatalf("m=%+v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	b.Publish(Message{ID: "evt_2"})
	b.Publish(Message{ID: "evt_3"})
	_, d := b.Stats()
	if d == 0 {
		t.Fatal("expected bounded-queue drop metric")
	}
}
