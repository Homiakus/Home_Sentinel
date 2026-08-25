package adapters

import (
	"encoding/json"
	"errors"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	"strings"
	"time"
)

type IntercomButtonPayload struct {
	DeviceID string `json:"device_id"`
	Location string `json:"location,omitempty"`
	Pressed  bool   `json:"pressed"`
	Sequence uint64 `json:"sequence,omitempty"`
}

func IntercomButton(topic string, raw []byte) (events.Envelope, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 || parts[0] != "sentinel" || parts[1] != "intercom" || parts[3] != "event" || parts[4] != "button" && parts[4] != "button_pressed" {
		return events.Envelope{}, errors.New("unexpected intercom button topic")
	}
	device := parts[2]
	var n struct {
		SchemaVersion int       `json:"schema_version"`
		Pressed       *bool     `json:"pressed"`
		Location      string    `json:"location"`
		Sequence      uint64    `json:"sequence"`
		OccurredAt    time.Time `json:"occurred_at"`
		CorrelationID string    `json:"correlation_id"`
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return events.Envelope{}, err
	}
	if n.SchemaVersion != 1 {
		return events.Envelope{}, errors.New("unsupported intercom event schema")
	}
	pressed := true
	if n.Pressed != nil {
		pressed = *n.Pressed
	}
	corr := domain.ID(n.CorrelationID)
	return envelope("intercom.button.pressed", "intercom:"+device, n.OccurredAt, events.SeverityInfo, IntercomButtonPayload{DeviceID: device, Location: n.Location, Pressed: pressed, Sequence: n.Sequence}, corr)
}
