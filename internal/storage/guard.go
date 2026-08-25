package storage

import "errors"

type Level string

const (
	Normal   Level = "NORMAL"
	Warning  Level = "WARNING"
	Critical Level = "CRITICAL"
	Unknown  Level = "UNKNOWN"
)

type Sample struct {
	Total uint64 `json:"total_bytes"`
	Free  uint64 `json:"free_bytes"`
}
type Guard struct {
	Policy Policy
	Level  Level
}

func (g *Guard) Evaluate(s Sample) (Level, error) {
	if err := g.Policy.Validate(); err != nil {
		return Unknown, err
	}
	if s.Total == 0 || s.Free > s.Total {
		return Unknown, errors.New("invalid storage sample")
	}
	pct := float64(s.Free) * 100 / float64(s.Total)
	critical := pct <= g.Policy.Thresholds.CriticalFreePercent || s.Free <= g.Policy.MinimumFreeBytes
	warning := pct <= g.Policy.Thresholds.WarningFreePercent
	if g.Level == Critical && !critical && pct < g.Policy.Thresholds.CriticalFreePercent+g.Policy.Thresholds.RecoveryHysteresisPercent {
		return Critical, nil
	}
	if g.Level == Warning && !warning && pct < g.Policy.Thresholds.WarningFreePercent+g.Policy.Thresholds.RecoveryHysteresisPercent {
		return Warning, nil
	}
	switch {
	case critical:
		g.Level = Critical
	case warning:
		g.Level = Warning
	default:
		g.Level = Normal
	}
	return g.Level, nil
}
