package model

import "fmt"

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

func (r RiskLevel) Validate() error {
	switch r {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return nil
	default:
		return fmt.Errorf("scenario: invalid risk level %q", r)
	}
}
