package action

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

type DoorRequest struct {
	RequestID   string            `json:"requestId"`
	DoorID      string            `json:"doorId"`
	Desired     gateway.LockState `json:"desired"`
	RequestedBy string            `json:"requestedBy,omitempty"`
	Reason      string            `json:"reason,omitempty"`
}

func (r DoorRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" {
		return errors.New("action: requestId is required")
	}
	if strings.TrimSpace(r.DoorID) == "" {
		return errors.New("action: doorId is required")
	}
	if r.Desired != gateway.LockLocked && r.Desired != gateway.LockUnlocked {
		return errors.New("action: desired lock state must be locked or unlocked")
	}
	return nil
}

func DoorExecutionID(r DoorRequest) string {
	key := r.RequestID + "\x00" + r.DoorID + "\x00" + string(r.Desired)
	sum := sha256.Sum256([]byte(key))
	return "door-action-" + hex.EncodeToString(sum[:16])
}

type ApprovalDecision string

const (
	ApprovalApprove ApprovalDecision = "approve"
	ApprovalReject  ApprovalDecision = "reject"
	ApprovalAbort   ApprovalDecision = "abort"
)

type ReconcileDecision string

const (
	ReconcileConfirm ReconcileDecision = "confirm"
	ReconcileRetry   ReconcileDecision = "retry"
	ReconcileAbort   ReconcileDecision = "abort"
)
