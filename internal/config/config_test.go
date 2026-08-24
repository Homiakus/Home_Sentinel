package config

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type mapSecrets map[string]string

func (s mapSecrets) Resolve(ctx context.Context, ref SecretRef) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, ok := s[ref.Env]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return []byte(value), nil
}

func TestDefaultConfigValidAndLoopbackOnlyWithoutTLS(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	cfg.Server.Listen = "0.0.0.0:8080"
	if err := cfg.Validate(); !errors.Is(err, ErrInsecureRemoteBind) {
		t.Fatalf("remote plaintext bind accepted: %v", err)
	}
	cfg.Server.TLS.Enabled = true
	cfg.Server.TLS.CertFile = "cert.pem"
	cfg.Server.TLS.KeyFile = "key.pem"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("remote TLS bind rejected: %v", err)
	}
}

func TestDecodeIsStrictAndBounded(t *testing.T) {
	if _, err := Decode(strings.NewReader(`{"version":1,"unknown":true}`), MaxConfigBytes); err == nil {
		t.Fatal("unknown config field accepted")
	}
	if _, err := Decode(strings.NewReader(strings.Repeat(" ", 65)), 64); err == nil {
		t.Fatal("oversized config accepted")
	}
	cfg, err := Decode(strings.NewReader(`{"version":1,"storage":{"root":"/srv/home-sentinel"}}`), MaxConfigBytes)
	if err != nil {
		t.Fatalf("decode partial config: %v", err)
	}
	if cfg.Storage.Root != "/srv/home-sentinel" || cfg.Server.Listen == "" {
		t.Fatalf("defaults were not preserved: %+v", cfg)
	}
}

func TestApplyEnvOverlaysReferencesNotSecretBytes(t *testing.T) {
	cfg := Default()
	values := map[string]string{
		"HOME_SENTINEL_LISTEN":                 "127.0.0.1:9090",
		"HOME_SENTINEL_DATA_ROOT":              "/data/sentinel",
		"HOME_SENTINEL_CALLBACK_ENABLED":       "true",
		"HOME_SENTINEL_CALLBACK_ACTIVE_KEY_ID": "k2",
	}
	if err := ApplyEnv(&cfg, func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Listen != "127.0.0.1:9090" || cfg.Storage.Root != "/data/sentinel" || !cfg.Security.Callbacks.Enabled || cfg.Security.Callbacks.ActiveKeyID != "k2" {
		t.Fatalf("env overlay failed: %+v", cfg)
	}
}

func TestCallbackKeyringResolvesSecretRefsAndSanitizedConfigDoesNotContainKey(t *testing.T) {
	cfg := Default()
	cfg.Security.Callbacks.Enabled = true
	cfg.Security.Callbacks.ActiveKeyID = "current"
	cfg.Security.Callbacks.Keys = map[string]SecretRef{
		"previous": {Env: "CALLBACK_PREVIOUS"},
		"current":  {Env: "CALLBACK_CURRENT"},
	}
	secrets := mapSecrets{
		"CALLBACK_PREVIOUS": strings.Repeat("p", 32),
		"CALLBACK_CURRENT":  strings.Repeat("c", 32),
	}
	ring, err := BuildCallbackKeyring(context.Background(), cfg.Security.Callbacks, secrets)
	if err != nil {
		t.Fatalf("build callback keyring: %v", err)
	}
	if ring == nil {
		t.Fatal("enabled callback keyring is nil")
	}

	sanitized, err := SanitizedJSON(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sanitized), strings.Repeat("c", 32)) || strings.Contains(string(sanitized), strings.Repeat("p", 32)) {
		t.Fatal("secret value leaked into sanitized config")
	}
	if !strings.Contains(string(sanitized), "CALLBACK_CURRENT") {
		t.Fatal("secret reference missing from diagnostics")
	}

	// Standard JSON marshaling is also safe because Config has no raw secret field.
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), strings.Repeat("c", 32)) {
		t.Fatal("secret value entered Config")
	}
}

func TestMissingSecretFailsClosedWithoutLeakingOtherValues(t *testing.T) {
	cfg := Default().Security.Callbacks
	cfg.Enabled = true
	cfg.ActiveKeyID = "current"
	cfg.Keys = map[string]SecretRef{"current": {Env: "MISSING_CALLBACK_KEY"}}
	_, err := BuildCallbackKeyring(context.Background(), cfg, mapSecrets{})
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("missing secret did not fail closed: %v", err)
	}
}
