package adapters

import (
	"encoding/json"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	"time"
)

func envelope(kind, source string, occurred time.Time, severity events.Severity, payload any, corr domain.ID) (events.Envelope, error) {
	id, err := domain.NewID("evt")
	if err != nil {
		return events.Envelope{}, err
	}
	if !corr.ValidFor("cor") {
		corr, err = domain.NewID("cor")
		if err != nil {
			return events.Envelope{}, err
		}
	}
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return events.Envelope{}, err
	}
	e := events.Envelope{SchemaVersion: events.SchemaV1, ID: id, Type: kind, Source: source, OccurredAt: occurred.UTC(), ReceivedAt: time.Now().UTC(), CorrelationID: corr, Severity: severity, Payload: b}
	return e, e.Validate()
}
