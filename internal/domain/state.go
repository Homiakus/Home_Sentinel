package domain

import "time"

type DesiredState[T any] struct {
	Revision int64     `json:"revision"`
	Value    T         `json:"value"`
	Updated  time.Time `json:"updated_at"`
}

type AppliedState[T any] struct {
	Revision int64     `json:"revision"`
	Checksum string    `json:"checksum"`
	Value    T         `json:"value"`
	Applied  time.Time `json:"applied_at"`
}

type ObservationStatus string

const (
	ObservationUnknown  ObservationStatus = "UNKNOWN"
	ObservationHealthy  ObservationStatus = "HEALTHY"
	ObservationDegraded ObservationStatus = "DEGRADED"
	ObservationFailed   ObservationStatus = "FAILED"
)

type ObservedState[T any] struct {
	Status     ObservationStatus `json:"status"`
	Value      T                 `json:"value"`
	ObservedAt time.Time         `json:"observed_at"`
	ReasonCode string            `json:"reason_code,omitempty"`
}
