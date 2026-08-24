package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

var ErrSecretNotFound = errors.New("config: referenced secret was not found")

type SecretSource interface {
	Resolve(context.Context, SecretRef) ([]byte, error)
}

type EnvSecretSource struct {
	Lookup LookupEnv
}

func (s EnvSecretSource) Resolve(ctx context.Context, ref SecretRef) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	lookup := s.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	value, ok := lookup(ref.Env)
	if !ok || value == "" {
		return nil, fmt.Errorf("%w: env=%s", ErrSecretNotFound, ref.Env)
	}
	return []byte(value), nil
}

// BuildCallbackKeyring resolves secret references just-in-time and wipes the
// temporary byte slices after callback.NewKeyring has copied them.
func BuildCallbackKeyring(ctx context.Context, cfg CallbackConfig, source SecretSource) (*callback.Keyring, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, errors.New("config: callback secret source is required")
	}
	keys := make(map[string][]byte, len(cfg.Keys))
	defer func() {
		for _, key := range keys {
			clear(key)
		}
	}()
	for id, ref := range cfg.Keys {
		value, err := source.Resolve(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("config: resolve callback key %q from %s: %w", id, strings.TrimSpace(ref.Env), err)
		}
		keys[id] = value
	}
	return callback.NewKeyring(cfg.ActiveKeyID, keys, cfg.Options())
}
