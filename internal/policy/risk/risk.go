package risk

import (
	"fmt"
	"math"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
)

const PolicyVersion = "risk-v2"

type Features struct {
	DetectionConfidence float64
	EvidenceCount       int
	PersonDetected      bool
	Identity            incident.IdentityState
	AlarmMode           string
	EntryActive         bool
	DwellSeconds        float64
	CrossCameraMatches  int
}

type Assessment struct {
	PolicyVersion string             `json:"policyVersion"`
	Risk          incident.Risk      `json:"risk"`
	Score         float64            `json:"score"`
	Contributions map[string]float64 `json:"contributions"`
	Reasons       []string           `json:"reasons"`
}

type Policy struct {
	MediumThreshold   float64
	HighThreshold     float64
	CriticalThreshold float64
}

func DefaultPolicy() Policy {
	return Policy{MediumThreshold: 0.35, HighThreshold: 0.60, CriticalThreshold: 0.82}
}

func FeaturesFromTrigger(trigger incident.Trigger, evidenceCount int) Features {
	return Features{
		DetectionConfidence: trigger.Confidence,
		EvidenceCount:       evidenceCount,
		PersonDetected:      strings.Contains(strings.ToLower(trigger.Kind), "person"),
		Identity:            trigger.Context.Identity,
		AlarmMode:           strings.ToLower(strings.TrimSpace(trigger.Context.AlarmMode)),
		EntryActive:         trigger.Context.EntryActive,
		DwellSeconds:        trigger.Context.DwellSeconds,
		CrossCameraMatches:  trigger.Context.CrossCameraMatches,
	}
}

func (p Policy) Assess(f Features) (Assessment, error) {
	if f.DetectionConfidence < 0 || f.DetectionConfidence > 1 {
		return Assessment{}, fmt.Errorf("risk: detection confidence must be in [0,1]")
	}
	if f.EvidenceCount < 0 || f.CrossCameraMatches < 0 || f.DwellSeconds < 0 {
		return Assessment{}, fmt.Errorf("risk: counts and duration must be non-negative")
	}
	if !(0 <= p.MediumThreshold && p.MediumThreshold < p.HighThreshold && p.HighThreshold < p.CriticalThreshold && p.CriticalThreshold <= 1) {
		return Assessment{}, fmt.Errorf("risk: invalid thresholds")
	}

	contrib := map[string]float64{}
	reasons := make([]string, 0, 8)

	contrib["detection_confidence"] = 0.30 * f.DetectionConfidence
	if f.PersonDetected {
		contrib["person_detected"] = 0.15
		reasons = append(reasons, "person detected")
	}
	switch f.Identity {
	case incident.IdentityUnknown:
		contrib["identity_unknown"] = 0.15
		reasons = append(reasons, "identity unknown")
	case incident.IdentityUncertain:
		contrib["identity_uncertain"] = 0.08
		reasons = append(reasons, "identity uncertain")
	}
	if f.AlarmMode == "away" || f.AlarmMode == "armed_away" {
		contrib["alarm_away"] = 0.10
		reasons = append(reasons, "alarm mode away")
	}
	if f.EntryActive {
		contrib["entry_active"] = 0.15
		reasons = append(reasons, "entry sensor active")
	}
	if f.DwellSeconds > 0 {
		contrib["dwell"] = 0.05 * math.Min(1, f.DwellSeconds/60)
		reasons = append(reasons, "non-zero dwell time")
	}
	if f.CrossCameraMatches > 0 {
		contrib["cross_camera"] = 0.05 * math.Min(1, float64(f.CrossCameraMatches)/2)
		reasons = append(reasons, "cross-camera continuity")
	}
	if f.EvidenceCount > 0 {
		contrib["evidence"] = 0.05 * math.Min(1, float64(f.EvidenceCount)/3)
	}

	score := 0.0
	for _, value := range contrib {
		score += value
	}
	score = math.Min(1, math.Max(0, score))

	level := incident.RiskLow
	switch {
	case score >= p.CriticalThreshold:
		level = incident.RiskCritical
	case score >= p.HighThreshold:
		level = incident.RiskHigh
	case score >= p.MediumThreshold:
		level = incident.RiskMedium
	}

	return Assessment{
		PolicyVersion: PolicyVersion,
		Risk:          level,
		Score:         score,
		Contributions: contrib,
		Reasons:       reasons,
	}, nil
}
