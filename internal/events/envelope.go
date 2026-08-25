package events

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

const SchemaV1 = 1

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

type Envelope struct {
	SchemaVersion int             `json:"schema_version"`
	ID            domain.ID       `json:"id"`
	Type          string          `json:"type"`
	Source        string          `json:"source"`
	OccurredAt    time.Time       `json:"occurred_at"`
	ReceivedAt    time.Time       `json:"received_at"`
	CorrelationID domain.ID       `json:"correlation_id"`
	CausationID   domain.ID       `json:"causation_id,omitempty"`
	Severity      Severity        `json:"severity"`
	Payload       json.RawMessage `json:"payload"`
}

func (e Envelope) Validate() error {
	if e.SchemaVersion != SchemaV1 {
		return errors.New("unsupported event schema")
	}
	if !e.ID.ValidFor("evt") {
		return errors.New("invalid event id")
	}
	if e.Type == "" || e.Source == "" {
		return errors.New("event type/source required")
	}
	if e.OccurredAt.IsZero() || e.ReceivedAt.IsZero() {
		return errors.New("event timestamps required")
	}
	if !e.CorrelationID.ValidFor("cor") {
		return errors.New("invalid correlation id")
	}
	if e.Severity == "" {
		return errors.New("severity required")
	}
	if !json.Valid(e.Payload) {
		return errors.New("payload must be valid json")
	}
	return nil
}
