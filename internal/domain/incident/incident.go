package incident

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/artifact"
)

type Risk string

const (
	RiskUnknown  Risk = "unknown"
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

type Status string

const (
	StatusOpen         Status = "open"
	StatusWaitingHuman Status = "waiting_human"
	StatusResolved     Status = "resolved"
	StatusArchived     Status = "archived"
)

type Decision string

const (
	DecisionAcknowledge Decision = "acknowledge"
	DecisionApprove     Decision = "approve"
	DecisionEdit        Decision = "edit"
	DecisionReject      Decision = "reject"
	DecisionRetry       Decision = "retry"
	DecisionConfirm     Decision = "confirm"
	DecisionAbort       Decision = "abort"
)

type IdentityState string

const (
	IdentityUnspecified IdentityState = ""
	IdentityKnown       IdentityState = "known"
	IdentityUnknown     IdentityState = "unknown"
	IdentityUncertain   IdentityState = "uncertain"
)

type SecurityContext struct {
	AlarmMode          string        `json:"alarmMode,omitempty"`
	Identity           IdentityState `json:"identity,omitempty"`
	EntryActive        bool          `json:"entryActive,omitempty"`
	DwellSeconds       float64       `json:"dwellSeconds,omitempty"`
	CrossCameraMatches int           `json:"crossCameraMatches,omitempty"`
}

type Trigger struct {
	EventID       string          `json:"eventId"`
	SourceID      string          `json:"sourceId"`
	Kind          string          `json:"kind"`
	OccurredAt    time.Time       `json:"occurredAt"`
	Confidence    float64         `json:"confidence,omitempty"`
	Artifacts     []artifact.Ref  `json:"artifacts,omitempty"`
	CorrelationID string          `json:"correlationId,omitempty"`
	Context       SecurityContext `json:"context,omitempty"`
}

func (t Trigger) Validate() error {
	if strings.TrimSpace(t.EventID) == "" || strings.TrimSpace(t.SourceID) == "" || strings.TrimSpace(t.Kind) == "" {
		return errors.New("incident: incomplete trigger identity")
	}
	if t.OccurredAt.IsZero() {
		return errors.New("incident: occurredAt is required")
	}
	if t.Confidence < 0 || t.Confidence > 1 {
		return fmt.Errorf("incident: confidence must be in [0,1], got %f", t.Confidence)
	}
	if t.Context.DwellSeconds < 0 {
		return errors.New("incident: dwellSeconds must be >= 0")
	}
	if t.Context.CrossCameraMatches < 0 {
		return errors.New("incident: crossCameraMatches must be >= 0")
	}
	for i, ref := range t.Artifacts {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("incident: artifact[%d]: %w", i, err)
		}
	}
	return nil
}

// ExecutionID is intentionally deterministic: redelivery of the same trigger
// must address the same ADGO execution rather than create another incident.
func ExecutionID(t Trigger) string {
	key := t.EventID + "\x00" + t.SourceID + "\x00" + t.Kind
	sum := sha256.Sum256([]byte(key))
	return "incident-" + hex.EncodeToString(sum[:16])
}
