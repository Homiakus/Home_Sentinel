package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/artifact"
)

var (
	ErrMissingID     = errors.New("event: missing id")
	ErrMissingKind   = errors.New("event: missing kind")
	ErrMissingSource = errors.New("event: missing source")
	ErrInvalidTime   = errors.New("event: invalid timestamps")
	ErrFutureEvent   = errors.New("event: occurred_at is after received_at beyond tolerated clock skew")
)

const MaxClockSkew = 5 * time.Minute

type Source struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Envelope is the transport-neutral contract accepted by the control plane.
// OccurredAt belongs to the producer clock, ReceivedAt to Home Sentinel.
type Envelope struct {
	ID            string          `json:"id"`
	Kind          string          `json:"kind"`
	Source        Source          `json:"source"`
	OccurredAt    time.Time       `json:"occurredAt"`
	ReceivedAt    time.Time       `json:"receivedAt"`
	CorrelationID string          `json:"correlationId,omitempty"`
	Artifacts     []artifact.Ref  `json:"artifacts,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

func (e Envelope) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return ErrMissingID
	}
	if strings.TrimSpace(e.Kind) == "" {
		return ErrMissingKind
	}
	if strings.TrimSpace(e.Source.ID) == "" || strings.TrimSpace(e.Source.Type) == "" {
		return ErrMissingSource
	}
	if e.OccurredAt.IsZero() || e.ReceivedAt.IsZero() {
		return ErrInvalidTime
	}
	if e.OccurredAt.After(e.ReceivedAt.Add(MaxClockSkew)) {
		return ErrFutureEvent
	}
	for i, ref := range e.Artifacts {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("event: artifact[%d]: %w", i, err)
		}
	}
	return nil
}
