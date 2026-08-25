package homeassistant

import (
	"context"
	"encoding/json"
	"testing"

	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
)

type recordPublisher struct{ messages []mqttint.Message }

func (p *recordPublisher) Publish(_ context.Context, msg mqttint.Message) error {
	msg.Payload = append([]byte(nil), msg.Payload...)
	p.messages = append(p.messages, msg)
	return nil
}

func TestDeviceDiscoveryRetainedAndStable(t *testing.T) {
	r := &recordPublisher{}
	p := DiscoveryPublisher{MQTT: r}
	devA, err := SentinelCameraDevice("cam_front", "Front Door", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	devB, err := SentinelCameraDevice("cam_front", "Вход", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	uidA := devA.Components["connectivity"]["unique_id"]
	uidB := devB.Components["connectivity"]["unique_id"]
	if uidA != uidB {
		t.Fatalf("rename changed unique id: %v != %v", uidA, uidB)
	}
	if err := p.PublishDevice(context.Background(), "home_sentinel_camera_cam_front", devA); err != nil {
		t.Fatal(err)
	}
	if len(r.messages) != 1 || !r.messages[0].Retained || r.messages[0].QoS != 1 {
		t.Fatalf("message=%+v", r.messages)
	}
	if r.messages[0].Topic != "homeassistant/device/home_sentinel_camera_cam_front/config" {
		t.Fatal(r.messages[0].Topic)
	}
	var raw map[string]any
	if err := json.Unmarshal(r.messages[0].Payload, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["dev"] == nil || raw["o"] == nil || raw["cmps"] == nil {
		t.Fatalf("payload=%v", raw)
	}
	if err := p.RemoveDevice(context.Background(), "home_sentinel_camera_cam_front"); err != nil {
		t.Fatal(err)
	}
	if len(r.messages) != 2 || len(r.messages[1].Payload) != 0 || !r.messages[1].Retained {
		t.Fatalf("remove=%+v", r.messages[1])
	}
}

func TestSentinelIntercomDeviceExposesObservedStateOnly(t *testing.T) {
	d, err := SentinelIntercomDevice("front_entry", "Front entry", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Components["door"]; !ok {
		t.Fatal("door component missing")
	}
	if _, ok := d.Components["lock_state"]; !ok {
		t.Fatal("lock state component missing")
	}
	for _, c := range d.Components {
		if _, ok := c["command_topic"]; ok {
			t.Fatal("Home Assistant must not bypass Sentinel unlock authorization")
		}
	}
}
