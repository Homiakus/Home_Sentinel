package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

type CameraRequest struct {
	RequestID string `json:"requestId"`
	CameraID  string `json:"cameraId"`
	Reason    string `json:"reason,omitempty"`
}

func (r CameraRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" {
		return errors.New("recovery: requestId is required")
	}
	if strings.TrimSpace(r.CameraID) == "" {
		return errors.New("recovery: cameraId is required")
	}
	return nil
}

func CameraExecutionID(r CameraRequest) string {
	sum := sha256.Sum256([]byte(r.RequestID + "\x00" + r.CameraID))
	return "camera-recovery-" + hex.EncodeToString(sum[:16])
}

type OperatorDecision string

const (
	OperatorRetry  OperatorDecision = "retry"
	OperatorReject OperatorDecision = "reject"
	OperatorAbort  OperatorDecision = "abort"
)
