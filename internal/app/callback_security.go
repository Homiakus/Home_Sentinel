package app

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

// CallbackSecurity is the narrow application-facing authority exposed to
// transports. HTTP code can authenticate an exact callback binding but cannot
// reach key material or mutate the keyring.
type CallbackSecurity interface {
	Accept(token string, expected callback.Binding) (callback.Claims, error)
	Sign(claims callback.Claims) (string, error)
}

func openCallbackSecurity(cfg config.CallbackRuntimeConfig, resolver secrets.Resolver) (CallbackSecurity, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	runtime, err := callback.OpenRuntime(callback.RuntimeConfig{
		ActiveKeyID:    cfg.ActiveKeyID,
		Keys:           cfg.Keys,
		Options:        cfg.Options(),
		ReplayCapacity: cfg.ReplayCapacity,
	}, resolver)
	if err != nil {
		return nil, fmt.Errorf("initialize callback security: %w", err)
	}
	return runtime, nil
}
