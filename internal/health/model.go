package health

import "time"

type Status string

const (
	Unknown  Status = "UNKNOWN"
	Starting Status = "STARTING"
	Healthy  Status = "HEALTHY"
	Degraded Status = "DEGRADED"
	Failed   Status = "FAILED"
)

type Component struct {
	Name       string    `json:"name"`
	Status     Status    `json:"status"`
	Since      time.Time `json:"since"`
	ReasonCode string    `json:"reason_code,omitempty"`
	Cause      string    `json:"cause,omitempty"`
}
