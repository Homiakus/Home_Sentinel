package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadCallbackRuntimeFromEnvironment(t *testing.T) {
	t.Setenv("SENTINEL_CALLBACK_ENABLED", "true")
	t.Setenv("SENTINEL_CALLBACK_ACTIVE_KEY_ID", "active")
	t.Setenv("SENTINEL_CALLBACK_KEYS", "old=secret://file/callback-old.key,active=secret://env/CALLBACK_ACTIVE")
	t.Setenv("SENTINEL_CALLBACK_MAX_TTL", "10m")
	t.Setenv("SENTINEL_CALLBACK_CLOCK_SKEW", "20s")
	t.Setenv("SENTINEL_CALLBACK_REPLAY_CAPACITY", "8192")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Security.Callbacks
	if !got.Enabled || got.ActiveKeyID != "active" || len(got.Keys) != 2 {
		t.Fatalf("callbacks=%+v", got)
	}
	if got.Keys["active"].String() != "secret://env/CALLBACK_ACTIVE" {
		t.Fatalf("active ref=%q", got.Keys["active"])
	}
	if got.MaxTTL != 10*time.Minute || got.ClockSkew != 20*time.Second || got.ReplayCapacity != 8192 {
		t.Fatalf("callback policy=%+v", got)
	}
}

func TestLoadCallbackRuntimeFileThenEnvironmentOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sentinel.json")
	content := `{
  "security": {
    "callbacks": {
      "enabled": true,
      "active_key_id": "file-key",
      "keys": {"file-key":"secret://file/callback-file.key"},
      "max_ttl": "12m",
      "clock_skew": "25s",
      "replay_capacity": 2048
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SENTINEL_CONFIG", path)
	t.Setenv("SENTINEL_CALLBACK_ACTIVE_KEY_ID", "env-key")
	t.Setenv("SENTINEL_CALLBACK_KEYS", "env-key=secret://env/CALLBACK_ENV")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Security.Callbacks
	if got.ActiveKeyID != "env-key" || len(got.Keys) != 1 || got.Keys["env-key"].String() != "secret://env/CALLBACK_ENV" {
		t.Fatalf("environment did not override callback key set: %+v", got)
	}
	if got.MaxTTL != 12*time.Minute || got.ReplayCapacity != 2048 {
		t.Fatalf("file callback policy lost after env override: %+v", got)
	}
}

func TestLoadCallbackRuntimeRejectsMalformedKeyMap(t *testing.T) {
	for _, raw := range []string{
		"missing-equals",
		"=secret://env/CALLBACK_ACTIVE",
		"active=",
		"active=secret://env/A,active=secret://env/B",
	} {
		t.Run(strings.ReplaceAll(raw, "/", "_"), func(t *testing.T) {
			t.Setenv("SENTINEL_CALLBACK_KEYS", raw)
			if _, err := Load(); err == nil {
				t.Fatalf("malformed callback key map %q accepted", raw)
			}
		})
	}
}

func TestLoadCallbackRuntimeRejectsRawSecretLikeReference(t *testing.T) {
	t.Setenv("SENTINEL_CALLBACK_ENABLED", "true")
	t.Setenv("SENTINEL_CALLBACK_ACTIVE_KEY_ID", "active")
	t.Setenv("SENTINEL_CALLBACK_KEYS", "active=this-is-not-a-secret-ref")
	if _, err := Load(); err == nil {
		t.Fatal("raw callback key value accepted as configuration")
	}
}
