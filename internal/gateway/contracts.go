package gateway

import (
	"context"
	"errors"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/domain/artifact"
)

var ErrMissingIdempotencyKey = errors.New("gateway: idempotency key is required")

type Operation struct {
	ExecutionID    string `json:"executionId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (o Operation) Validate() error {
	if strings.TrimSpace(o.IdempotencyKey) == "" {
		return ErrMissingIdempotencyKey
	}
	return nil
}

type EffectState string

const (
	EffectApplied        EffectState = "applied"
	EffectAlreadyApplied EffectState = "already_applied"
	EffectAmbiguous      EffectState = "ambiguous"
)

type EffectResult struct {
	State       EffectState `json:"state"`
	ProviderID  string      `json:"providerId,omitempty"`
	Description string      `json:"description,omitempty"`
}

type Notification struct {
	Channel string         `json:"channel"`
	Title   string         `json:"title"`
	Body    string         `json:"body"`
	Media   []artifact.Ref `json:"media,omitempty"`
}

type Notifier interface {
	Notify(context.Context, Operation, Notification) (EffectResult, error)
}

type LockState string

const (
	LockLocked   LockState = "locked"
	LockUnlocked LockState = "unlocked"
	LockUnknown  LockState = "unknown"
)

type DoorController interface {
	LockState(context.Context, string) (LockState, error)
	SetLockState(context.Context, Operation, string, LockState) (EffectResult, error)
}

type SirenController interface {
	Enabled(context.Context, string) (bool, error)
	SetEnabled(context.Context, Operation, string, bool) (EffectResult, error)
}

type ArtifactStore interface {
	Put(context.Context, Operation, string, []byte) (artifact.Ref, error)
	Get(context.Context, artifact.Ref) ([]byte, error)
}
