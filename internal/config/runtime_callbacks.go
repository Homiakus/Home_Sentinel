package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

const maxRuntimeCallbackKeys = 16

// CallbackRuntimeConfig is the runtime-facing callback policy. Keys are opaque
// Stage-16 secret references; secret bytes are resolved only while app.Open is
// constructing the callback keyring.
type CallbackRuntimeConfig struct {
	Enabled        bool
	ActiveKeyID    string
	Keys           map[string]secrets.Ref
	MaxTTL         time.Duration
	ClockSkew      time.Duration
	ReplayCapacity int
}

func DefaultCallbackRuntimeConfig() CallbackRuntimeConfig {
	return CallbackRuntimeConfig{
		MaxTTL:         callback.DefaultOptions.MaxTTL,
		ClockSkew:      callback.DefaultOptions.ClockSkew,
		ReplayCapacity: 4096,
	}
}

func (c CallbackRuntimeConfig) Validate() error {
	if c.MaxTTL <= 0 || c.MaxTTL > time.Hour {
		return errors.New("callback runtime max TTL must be between 1ns and 1h")
	}
	if c.ClockSkew < 0 || c.ClockSkew > 5*time.Minute {
		return errors.New("callback runtime clock skew must be between 0 and 5m")
	}
	if c.ReplayCapacity <= 0 || c.ReplayCapacity > 1_000_000 {
		return errors.New("callback runtime replay capacity must be between 1 and 1000000")
	}
	if len(c.Keys) > maxRuntimeCallbackKeys {
		return fmt.Errorf("callback runtime supports at most %d overlapping keys", maxRuntimeCallbackKeys)
	}
	for id, ref := range c.Keys {
		if !validRuntimeCallbackKeyID(id) {
			return fmt.Errorf("callback runtime key id %q is invalid", id)
		}
		if err := validateRuntimeSecretRef(ref); err != nil {
			return fmt.Errorf("callback runtime key %q: %w", id, err)
		}
	}
	if !c.Enabled {
		return nil
	}
	if !validRuntimeCallbackKeyID(c.ActiveKeyID) {
		return errors.New("callback runtime active key id is required and must be valid when callbacks are enabled")
	}
	if len(c.Keys) == 0 {
		return errors.New("callback runtime keys are required when callbacks are enabled")
	}
	if _, ok := c.Keys[c.ActiveKeyID]; !ok {
		return errors.New("callback runtime active key id is not present in keys")
	}
	return nil
}

func (c CallbackRuntimeConfig) Options() callback.Options {
	return callback.Options{MaxTTL: c.MaxTTL, ClockSkew: c.ClockSkew}
}

func validateRuntimeSecretRef(ref secrets.Ref) error {
	raw := strings.TrimSpace(ref.String())
	if _, err := secrets.ParseRef(raw); err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(raw, "secret://env/"):
		name := strings.TrimPrefix(raw, "secret://env/")
		if name == "" || strings.ContainsAny(name, "/=\\") {
			return errors.New("invalid environment secret reference")
		}
	case strings.HasPrefix(raw, "secret://file/"):
		if strings.TrimSpace(strings.TrimPrefix(raw, "secret://file/")) == "" {
			return errors.New("empty file secret reference")
		}
	default:
		return errors.New("callback runtime secret provider must be env or file")
	}
	return nil
}

func validRuntimeCallbackKeyID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
