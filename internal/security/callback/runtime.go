package callback

import (
	"fmt"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

// SecretResolver is the narrow Stage-16 boundary required by callback runtime
// bootstrap. Implementations return secret bytes for opaque refs and must not
// persist resolved material in configuration objects.
type SecretResolver interface {
	Resolve(secrets.Ref) ([]byte, error)
}

// RuntimeConfig contains only key identifiers, secret references and bounded
// replay/TTL policy. It never contains signing-key bytes.
type RuntimeConfig struct {
	ActiveKeyID    string
	Keys           map[string]secrets.Ref
	Options        Options
	ReplayCapacity int
}

// Runtime owns callback signing/verification state after bootstrap. Keyring
// keeps private copies of resolved keys; temporary resolver buffers are wiped
// before OpenRuntime returns.
type Runtime struct {
	keyring  *Keyring
	acceptor *Acceptor
}

func OpenRuntime(cfg RuntimeConfig, resolver SecretResolver) (*Runtime, error) {
	if resolver == nil {
		return nil, fmt.Errorf("callback: secret resolver is required")
	}
	if len(cfg.Keys) == 0 {
		return nil, fmt.Errorf("callback: at least one key reference is required")
	}
	material := make(map[string][]byte, len(cfg.Keys))
	defer wipeKeyMaterial(material)
	for id, ref := range cfg.Keys {
		key, err := resolver.Resolve(ref)
		if err != nil {
			return nil, fmt.Errorf("callback: resolve key %q: %w", id, err)
		}
		material[id] = key
	}
	keyring, err := NewKeyring(cfg.ActiveKeyID, material, cfg.Options)
	if err != nil {
		return nil, err
	}
	replay, err := NewReplayGuard(cfg.ReplayCapacity)
	if err != nil {
		return nil, err
	}
	acceptor, err := NewAcceptor(keyring, replay)
	if err != nil {
		return nil, err
	}
	return &Runtime{keyring: keyring, acceptor: acceptor}, nil
}

func (r *Runtime) Sign(claims Claims) (string, error) {
	if r == nil || r.keyring == nil {
		return "", ErrInvalidKey
	}
	return r.keyring.Sign(claims)
}

func (r *Runtime) Accept(token string, expected Binding) (Claims, error) {
	if r == nil || r.acceptor == nil {
		return Claims{}, fmt.Errorf("callback: runtime is not configured")
	}
	return r.acceptor.Accept(token, expected)
}

func wipeKeyMaterial(keys map[string][]byte) {
	for _, key := range keys {
		for i := range key {
			key[i] = 0
		}
	}
}
