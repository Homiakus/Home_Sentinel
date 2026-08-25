package intercom

import (
	"encoding/json"
	"errors"
	"time"
)

const SchemaVersion = 1

type Command struct {
	SchemaVersion int       `json:"schema_version"`
	RequestID     string    `json:"request_id"`
	CorrelationID string    `json:"correlation_id"`
	Action        string    `json:"action"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type Ack struct {
	SchemaVersion int       `json:"schema_version"`
	RequestID     string    `json:"request_id"`
	Accepted      bool      `json:"accepted"`
	Reason        string    `json:"reason,omitempty"`
	ObservedAt    time.Time `json:"observed_at"`
}

type Result struct {
	SchemaVersion int       `json:"schema_version"`
	RequestID     string    `json:"request_id"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
	ObservedAt    time.Time `json:"observed_at"`
}

type StatePayload struct {
	SchemaVersion int       `json:"schema_version"`
	Value         string    `json:"value"`
	Sequence      uint64    `json:"sequence,omitempty"`
	ObservedAt    time.Time `json:"observed_at"`
}

func decodeVersioned[T any](raw []byte, out *T) error {
	if len(raw) == 0 {
		return errors.New("empty intercom payload")
	}
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return err
	}
	if head.SchemaVersion != SchemaVersion {
		return errors.New("unsupported intercom schema version")
	}
	return json.Unmarshal(raw, out)
}
