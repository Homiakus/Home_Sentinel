package hardware

import (
	"context"
	"encoding/json"
	"errors"
)

type SMARTStatus struct {
	Device       string `json:"device"`
	Available    bool   `json:"available"`
	Passed       *bool  `json:"passed,omitempty"`
	TemperatureC *int   `json:"temperature_c,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

func ProbeSMART(ctx context.Context, r Runner, device string) SMARTStatus {
	s := SMARTStatus{Device: device}
	if r == nil {
		s.Reason = "SMART runner unavailable"
		return s
	}
	out, err := r.Run(ctx, "smartctl", "-j", "-H", "-A", device)
	if err != nil {
		s.Reason = err.Error()
		return s
	}
	var doc struct {
		SmartStatus *struct {
			Passed bool `json:"passed"`
		} `json:"smart_status"`
		Temperature *struct {
			Current int `json:"current"`
		} `json:"temperature"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		s.Reason = "invalid smartctl JSON"
		return s
	}
	s.Available = true
	if doc.SmartStatus != nil {
		v := doc.SmartStatus.Passed
		s.Passed = &v
	}
	if doc.Temperature != nil {
		v := doc.Temperature.Current
		s.TemperatureC = &v
	}
	return s
}

var _ = errors.New
