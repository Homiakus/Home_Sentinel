package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const maxIdentifierBytes = 160

type ID string
type RevisionID string
type StepID string

func NewID(prefix string) (ID, error) {
	prefix = strings.TrimSpace(prefix)
	if err := validateToken("id prefix", prefix); err != nil {
		return "", err
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("scenario: generate id: %w", err)
	}
	return ID(prefix + "-" + hex.EncodeToString(raw[:])), nil
}

func NewRevisionID() (RevisionID, error) {
	id, err := NewID("rev")
	return RevisionID(id), err
}

func validateToken(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("scenario: %s is required", label)
	}
	if len(value) > maxIdentifierBytes {
		return fmt.Errorf("scenario: %s exceeds %d bytes", label, maxIdentifierBytes)
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '-', '_', '.', ':', '/':
			continue
		default:
			return errors.New("scenario: identifiers may contain only ASCII letters, digits, '-', '_', '.', ':', '/'")
		}
	}
	return nil
}
