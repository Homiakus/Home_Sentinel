package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

func TestCallbackRuntimeKeyCountBoundary(t *testing.T) {
	cfg := DefaultCallbackRuntimeConfig()
	cfg.Enabled = true
	cfg.ActiveKeyID = "key-00"
	cfg.Keys = map[string]secrets.Ref{}
	for i := 0; i < maxRuntimeCallbackKeys; i++ {
		id := fmt.Sprintf("key-%02d", i)
		cfg.Keys[id] = secrets.Ref("secret://env/CALLBACK_" + fmt.Sprintf("%02d", i))
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("maximum reviewed overlap rejected: %v", err)
	}
	cfg.Keys["key-overflow"] = secrets.Ref("secret://env/CALLBACK_OVERFLOW")
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("overflow key set error=%v", err)
	}
}

func TestCallbackRuntimeKeyIDCharacterAndLengthBoundary(t *testing.T) {
	for _, id := range []string{"a", strings.Repeat("z", 64), "A-Z_0.9"} {
		cfg := DefaultCallbackRuntimeConfig()
		cfg.Enabled = true
		cfg.ActiveKeyID = id
		cfg.Keys = map[string]secrets.Ref{id: secrets.Ref("secret://env/CALLBACK_ACTIVE")}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid key id %q rejected: %v", id, err)
		}
	}
	for _, id := range []string{"", strings.Repeat("z", 65), "bad/key", "bad key", "bad:key"} {
		cfg := DefaultCallbackRuntimeConfig()
		cfg.Enabled = true
		cfg.ActiveKeyID = id
		cfg.Keys = map[string]secrets.Ref{id: secrets.Ref("secret://env/CALLBACK_ACTIVE")}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("invalid key id %q accepted", id)
		}
	}
}
