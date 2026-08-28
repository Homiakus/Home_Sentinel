package config

import (
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

func TestDefaultCallbackRuntimeDisabledAndValid(t *testing.T) {
	cfg := DefaultCallbackRuntimeConfig()
	if cfg.Enabled {
		t.Fatal("callbacks must be disabled by default")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default callback runtime invalid: %v", err)
	}
}

func TestCallbackRuntimeEnabledRequiresActiveReferencedKey(t *testing.T) {
	base := DefaultCallbackRuntimeConfig()
	base.Enabled = true

	if err := base.Validate(); err == nil {
		t.Fatal("enabled callback runtime without key accepted")
	}

	base.ActiveKeyID = "active"
	base.Keys = map[string]secrets.Ref{"other": secrets.Ref("secret://env/CALLBACK_OTHER")}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("missing active key produced wrong result: %v", err)
	}

	base.Keys["active"] = secrets.Ref("secret://env/CALLBACK_ACTIVE")
	if err := base.Validate(); err != nil {
		t.Fatalf("valid enabled callback runtime rejected: %v", err)
	}
}

func TestCallbackRuntimeRejectsInvalidRefsAndPolicyBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CallbackRuntimeConfig)
	}{
		{name: "bad key id", mutate: func(c *CallbackRuntimeConfig) {
			c.ActiveKeyID = "bad/key"
			c.Keys = map[string]secrets.Ref{"bad/key": secrets.Ref("secret://env/CALLBACK_ACTIVE")}
		}},
		{name: "raw-looking provider", mutate: func(c *CallbackRuntimeConfig) {
			c.Keys = map[string]secrets.Ref{"active": secrets.Ref("secret://literal/do-not-store-secret")}
		}},
		{name: "empty env ref", mutate: func(c *CallbackRuntimeConfig) {
			c.Keys = map[string]secrets.Ref{"active": secrets.Ref("secret://env/")}
		}},
		{name: "ttl zero", mutate: func(c *CallbackRuntimeConfig) { c.MaxTTL = 0 }},
		{name: "ttl too large", mutate: func(c *CallbackRuntimeConfig) { c.MaxTTL = time.Hour + time.Nanosecond }},
		{name: "negative skew", mutate: func(c *CallbackRuntimeConfig) { c.ClockSkew = -time.Nanosecond }},
		{name: "skew too large", mutate: func(c *CallbackRuntimeConfig) { c.ClockSkew = 5*time.Minute + time.Nanosecond }},
		{name: "replay zero", mutate: func(c *CallbackRuntimeConfig) { c.ReplayCapacity = 0 }},
		{name: "replay too large", mutate: func(c *CallbackRuntimeConfig) { c.ReplayCapacity = 1_000_001 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultCallbackRuntimeConfig()
			cfg.Enabled = true
			cfg.ActiveKeyID = "active"
			cfg.Keys = map[string]secrets.Ref{"active": secrets.Ref("secret://env/CALLBACK_ACTIVE")}
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("invalid callback runtime accepted: %+v", cfg)
			}
		})
	}
}

func TestCallbackRuntimeAcceptsInclusivePolicyBoundaries(t *testing.T) {
	for _, tc := range []CallbackRuntimeConfig{
		{
			Enabled: true, ActiveKeyID: "active",
			Keys: map[string]secrets.Ref{"active": secrets.Ref("secret://file/callback-active.key")},
			MaxTTL: time.Nanosecond, ClockSkew: 0, ReplayCapacity: 1,
		},
		{
			Enabled: true, ActiveKeyID: "active",
			Keys: map[string]secrets.Ref{"active": secrets.Ref("secret://env/CALLBACK_ACTIVE")},
			MaxTTL: time.Hour, ClockSkew: 5 * time.Minute, ReplayCapacity: 1_000_000,
		},
	} {
		if err := tc.Validate(); err != nil {
			t.Fatalf("inclusive callback policy boundary rejected: %v", err)
		}
	}
}
