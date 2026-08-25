package intercom

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	eventadapters "github.com/Homiakus/Home_Sentinel/internal/events/adapters"
	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
	"github.com/Homiakus/Home_Sentinel/internal/locks"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type Service struct {
	Devices  *repository.Store[Device]
	States   *repository.Store[ObservedState]
	Commands CommandStore
	MQTT     MQTTPublisher
	Events   *events.Bus
	Audit    *repository.AuditStore
	Locks    *locks.Manager
	Now      func() time.Time
	Cameras  *cameras.Service
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) Put(ctx context.Context, d Device) (Device, error) {
	if err := d.Validate(); err != nil {
		return Device{}, err
	}
	if d.CameraID != "" && s.Cameras != nil {
		if _, err := s.Cameras.Get(ctx, d.CameraID); err != nil {
			return Device{}, errors.New("linked camera not found")
		}
	}
	r, err := s.Devices.Put(ctx, d.ID, d)
	return r.Value, err
}

func (s *Service) Get(ctx context.Context, id string) (Device, error) {
	r, err := s.Devices.Get(ctx, id)
	return r.Value, err
}
func (s *Service) List(ctx context.Context, limit int) ([]Device, error) {
	rs, err := s.Devices.List(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Value)
	}
	return out, nil
}
func (s *Service) State(ctx context.Context, id string) (ObservedState, error) {
	r, err := s.States.Get(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return ObservedState{DeviceID: id, Door: DoorUnknown, Lock: LockUnknown}, nil
	}
	return r.Value, err
}

func (s *Service) IngestMQTT(ctx context.Context, msg mqttint.Message) error {
	parts := strings.Split(msg.Topic, "/")
	if len(parts) != 5 || parts[0] != "sentinel" || parts[1] != "intercom" {
		return errors.New("unexpected intercom topic")
	}
	deviceID, kind, key := parts[2], parts[3], parts[4]
	if !validStableID(deviceID) {
		return errors.New("invalid intercom device id")
	}
	if kind == "event" && key == "button" {
		var b struct {
			SchemaVersion int       `json:"schema_version"`
			Sequence      uint64    `json:"sequence"`
			OccurredAt    time.Time `json:"occurred_at"`
		}
		if err := json.Unmarshal(msg.Payload, &b); err != nil {
			return err
		}
		if b.SchemaVersion != SchemaVersion {
			return errors.New("unsupported intercom schema version")
		}
		now := s.now()
		at := b.OccurredAt
		if at.IsZero() {
			at = now
		}
		if at.After(now.Add(5 * time.Minute)) {
			return errors.New("intercom button timestamp is too far in the future")
		}
		release := func() {}
		if s.Locks != nil {
			release = s.Locks.Lock("intercom-state:" + deviceID)
		}
		defer release()
		st, err := s.State(ctx, deviceID)
		if err != nil {
			return err
		}
		if b.Sequence > 0 && b.Sequence <= st.LastButtonSequence {
			return nil
		}
		if b.Sequence == 0 && !st.LastButtonAt.IsZero() {
			d := at.Sub(st.LastButtonAt)
			if d >= 0 && d < 750*time.Millisecond {
				return nil
			}
		}
		st.DeviceID = deviceID
		st.LastButtonAt = at
		if b.Sequence > st.LastButtonSequence {
			st.LastButtonSequence = b.Sequence
		}
		st.LastSeenAt = at
		st.UpdatedAt = now
		if _, err := s.States.Put(ctx, deviceID, st); err != nil {
			return err
		}
		e, err := eventadapters.IntercomButton(msg.Topic, msg.Payload)
		if err != nil {
			return err
		}
		if s.Events != nil {
			s.Events.Publish(e)
		}
		return nil
	}
	if kind == "event" && key == "ack" {
		var ack Ack
		if err := decodeVersioned(msg.Payload, &ack); err != nil {
			return err
		}
		return s.Commands.UpdateAck(ctx, deviceID, ack, s.now())
	}
	if kind == "event" && key == "result" {
		var result Result
		if err := decodeVersioned(msg.Payload, &result); err != nil {
			return err
		}
		return s.Commands.UpdateResult(ctx, deviceID, result, s.now())
	}
	if kind != "state" {
		return errors.New("unsupported intercom MQTT message")
	}
	var state StatePayload
	if err := decodeVersioned(msg.Payload, &state); err != nil {
		return err
	}
	release := func() {}
	if s.Locks != nil {
		release = s.Locks.Lock("intercom-state:" + deviceID)
	}
	defer release()
	st, err := s.State(ctx, deviceID)
	if err != nil {
		return err
	}
	now := s.now()
	observed := state.ObservedAt
	if observed.IsZero() {
		observed = now
	}
	if observed.After(now.Add(5 * time.Minute)) {
		return errors.New("intercom state timestamp is too far in the future")
	}
	if st.Sequences == nil {
		st.Sequences = map[string]uint64{}
	}
	if state.Sequence != 0 && state.Sequence <= st.Sequences[key] {
		return nil
	}
	st.DeviceID = deviceID
	st.LastSeenAt = observed
	st.UpdatedAt = now
	if state.Sequence > st.Sequences[key] {
		st.Sequences[key] = state.Sequence
	}
	switch key {
	case "availability":
		st.Available = strings.EqualFold(state.Value, "online")
	case "door":
		switch strings.ToLower(state.Value) {
		case "open":
			st.Door = DoorOpen
		case "closed":
			st.Door = DoorClosed
		default:
			st.Door = DoorUnknown
		}
	case "lock":
		switch strings.ToLower(state.Value) {
		case "locked":
			st.Lock = LockLocked
		case "unlocked":
			st.Lock = LockUnlocked
		default:
			st.Lock = LockUnknown
		}
	default:
		return errors.New("unsupported intercom state key")
	}
	_, err = s.States.Put(ctx, deviceID, st)
	return err
}

func EncodeState(value string, seq uint64, at time.Time) []byte {
	b, _ := json.Marshal(StatePayload{SchemaVersion: SchemaVersion, Value: value, Sequence: seq, ObservedAt: at})
	return b
}
