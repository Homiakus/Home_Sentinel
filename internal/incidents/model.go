package incidents

import (
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"time"
)

type State string

const (
	StateOpen   State = "open"
	StateClosed State = "closed"
)

type Incident struct {
	ID             domain.ID   `json:"id"`
	State          State       `json:"state"`
	OpenedAt       time.Time   `json:"opened_at"`
	LastEventAt    time.Time   `json:"last_event_at"`
	Location       string      `json:"location,omitempty"`
	CameraID       string      `json:"camera_id,omitempty"`
	CorrelationIDs []domain.ID `json:"correlation_ids,omitempty"`
	ObjectIDs      []string    `json:"object_ids,omitempty"`
	EventIDs       []domain.ID `json:"event_ids"`
}
