package observation

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/artifact"
	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
)

type Observation struct {
	EventID    string                   `json:"eventId"`
	SubjectKey string                   `json:"subjectKey"`
	SourceID   string                   `json:"sourceId"`
	Kind       string                   `json:"kind"`
	OccurredAt time.Time                `json:"occurredAt"`
	ReceivedAt time.Time                `json:"receivedAt"`
	Confidence float64                  `json:"confidence,omitempty"`
	Artifacts  []artifact.Ref           `json:"artifacts,omitempty"`
	Context    incident.SecurityContext `json:"context,omitempty"`
}

func (o Observation) Validate() error {
	if strings.TrimSpace(o.EventID) == "" {
		return errors.New("observation: eventId is required")
	}
	if strings.TrimSpace(o.SubjectKey) == "" {
		return errors.New("observation: subjectKey is required")
	}
	if strings.TrimSpace(o.SourceID) == "" || strings.TrimSpace(o.Kind) == "" {
		return errors.New("observation: sourceId and kind are required")
	}
	if o.OccurredAt.IsZero() || o.ReceivedAt.IsZero() {
		return errors.New("observation: occurredAt and receivedAt are required")
	}
	if o.Confidence < 0 || o.Confidence > 1 {
		return fmt.Errorf("observation: confidence must be in [0,1]")
	}
	if o.Context.DwellSeconds < 0 || o.Context.CrossCameraMatches < 0 {
		return errors.New("observation: context counts/duration must be non-negative")
	}
	for i, ref := range o.Artifacts {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("observation: artifact[%d]: %w", i, err)
		}
	}
	return nil
}
