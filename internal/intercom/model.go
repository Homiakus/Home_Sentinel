package intercom

import (
	"errors"
	"strings"
	"time"
)

type DoorState string
type LockState string

const (
	DoorUnknown DoorState = "unknown"
	DoorClosed  DoorState = "closed"
	DoorOpen    DoorState = "open"

	LockUnknown  LockState = "unknown"
	LockLocked   LockState = "locked"
	LockUnlocked LockState = "unlocked"
)

type Capabilities struct {
	Button     bool `json:"button"`
	DoorSensor bool `json:"door_sensor"`
	Lock       bool `json:"lock"`
	Audio      bool `json:"audio"`
	Talk       bool `json:"talk"`
}

type Device struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Location     string       `json:"location,omitempty"`
	CameraID     string       `json:"camera_id,omitempty"`
	Capabilities Capabilities `json:"capabilities"`
}

func (d Device) Validate() error {
	if !validStableID(d.ID) {
		return errors.New("invalid intercom id")
	}
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("intercom name required")
	}
	if d.CameraID != "" && !validStableID(d.CameraID) {
		return errors.New("invalid intercom camera id")
	}
	return nil
}

type ObservedState struct {
	DeviceID           string            `json:"device_id"`
	Available          bool              `json:"available"`
	Door               DoorState         `json:"door"`
	Lock               LockState         `json:"lock"`
	Sequences          map[string]uint64 `json:"sequences,omitempty"`
	LastButtonSequence uint64            `json:"last_button_sequence,omitempty"`
	LastButtonAt       time.Time         `json:"last_button_at,omitempty"`
	LastSeenAt         time.Time         `json:"last_seen_at,omitempty"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func (s ObservedState) Fresh(now time.Time, maxAge time.Duration) bool {
	return s.Available && !s.LastSeenAt.IsZero() && now.Sub(s.LastSeenAt) <= maxAge
}

func validStableID(v string) bool {
	if len(v) == 0 || len(v) > 128 {
		return false
	}
	for i, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			if i == 0 && (r == '_' || r == '-') {
				return false
			}
			continue
		}
		return false
	}
	return true
}
