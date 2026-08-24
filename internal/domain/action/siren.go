package action

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

type SirenRequest struct {
	RequestID   string `json:"requestId"`
	SirenID     string `json:"sirenId"`
	RequestedBy string `json:"requestedBy,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func (r SirenRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" {
		return errors.New("action: requestId is required")
	}
	if strings.TrimSpace(r.SirenID) == "" {
		return errors.New("action: sirenId is required")
	}
	return nil
}

func SirenExecutionID(r SirenRequest) string {
	sum := sha256.Sum256([]byte(r.RequestID + "\x00" + r.SirenID))
	return "siren-action-" + hex.EncodeToString(sum[:16])
}
