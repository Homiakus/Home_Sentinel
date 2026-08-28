package callback

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidKey   = errors.New("callback: signing key must contain at least 32 bytes")
	ErrInvalidToken = errors.New("callback: invalid token")
	ErrExpired      = errors.New("callback: token expired")
	ErrNotYetValid  = errors.New("callback: token issued in the future")
	ErrTTLExceeded  = errors.New("callback: token ttl exceeds policy")
	ErrUnknownKey   = errors.New("callback: unknown signing key")
)

const (
	MinKeyBytes   = 32
	MaxTokenBytes = 4096
)

var DefaultOptions = Options{
	MaxTTL:    15 * time.Minute,
	ClockSkew: 30 * time.Second,
}

type Options struct {
	MaxTTL    time.Duration
	ClockSkew time.Duration
}

func normalizeOptions(options Options) Options {
	if options.MaxTTL <= 0 {
		options.MaxTTL = DefaultOptions.MaxTTL
	}
	if options.ClockSkew < 0 {
		options.ClockSkew = 0
	}
	return options
}

type Claims struct {
	KeyID       string `json:"keyId"`
	ExecutionID string `json:"executionId"`
	NodeID      string `json:"nodeId"`
	EventID     string `json:"eventId"`
	Subject     string `json:"subject,omitempty"`
	// Action binds a Stage-17 ingress token to the exact callback operation it
	// may authorize. It remains optional at the Stage-12 crypto primitive so
	// previously issued/internal tokens can still be verified; Stage-17
	// Acceptor requires an exact non-empty action binding.
	Action    string `json:"action,omitempty"`
	Nonce     string `json:"nonce"`
	IssuedAt  int64  `json:"issuedAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (c Claims) Validate(now time.Time) error {
	return c.validate(now, DefaultOptions)
}

func (c Claims) validate(now time.Time, options Options) error {
	options = normalizeOptions(options)
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !validKeyID(c.KeyID) || strings.TrimSpace(c.ExecutionID) == "" || strings.TrimSpace(c.NodeID) == "" || strings.TrimSpace(c.EventID) == "" || strings.TrimSpace(c.Nonce) == "" {
		return fmt.Errorf("%w: incomplete claims", ErrInvalidToken)
	}
	if c.IssuedAt <= 0 || c.ExpiresAt <= c.IssuedAt {
		return fmt.Errorf("%w: invalid validity interval", ErrInvalidToken)
	}
	issuedAt := time.Unix(c.IssuedAt, 0).UTC()
	expiresAt := time.Unix(c.ExpiresAt, 0).UTC()
	if issuedAt.After(now.Add(options.ClockSkew)) {
		return ErrNotYetValid
	}
	if !expiresAt.After(now.Add(-options.ClockSkew)) {
		return ErrExpired
	}
	if expiresAt.Sub(issuedAt) > options.MaxTTL {
		return ErrTTLExceeded
	}
	return nil
}

func validKeyID(value string) bool {
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

type Signer struct {
	keyID   string
	key     []byte
	now     func() time.Time
	options Options
}

func NewSigner(key []byte) (*Signer, error) {
	return NewSignerWithID("default", key, DefaultOptions)
}

func NewSignerWithID(keyID string, key []byte, options Options) (*Signer, error) {
	if !validKeyID(keyID) {
		return nil, fmt.Errorf("%w: invalid key id", ErrInvalidKey)
	}
	if len(key) < MinKeyBytes {
		return nil, ErrInvalidKey
	}
	return &Signer{
		keyID:   keyID,
		key:     append([]byte(nil), key...),
		now:     time.Now,
		options: normalizeOptions(options),
	}, nil
}

func (s *Signer) Sign(claims Claims) (string, error) {
	if s == nil || len(s.key) < MinKeyBytes || !validKeyID(s.keyID) {
		return "", ErrInvalidKey
	}
	now := s.now().UTC()
	if claims.KeyID == "" {
		claims.KeyID = s.keyID
	}
	if claims.KeyID != s.keyID {
		return "", fmt.Errorf("%w: claims key id does not match signer", ErrUnknownKey)
	}
	if claims.IssuedAt == 0 {
		claims.IssuedAt = now.Unix()
	}
	if err := claims.validate(now, s.options); err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	message := s.keyID + "." + encoded
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token := message + "." + signature
	if len(token) > MaxTokenBytes {
		return "", ErrInvalidToken
	}
	return token, nil
}

func (s *Signer) Verify(token string) (Claims, error) {
	if s == nil || len(s.key) < MinKeyBytes {
		return Claims{}, ErrInvalidKey
	}
	return verifyWithKey(token, s.keyID, s.key, s.now().UTC(), s.options)
}

type Keyring struct {
	activeID string
	keys     map[string][]byte
	now      func() time.Time
	options  Options
}

// NewKeyring supports overlap during key rotation: Sign always uses activeID,
// while Verify accepts every configured key ID. Removing an old key retires it.
func NewKeyring(activeID string, keys map[string][]byte, options Options) (*Keyring, error) {
	if !validKeyID(activeID) {
		return nil, fmt.Errorf("%w: invalid active key id", ErrInvalidKey)
	}
	copied := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if !validKeyID(id) || len(key) < MinKeyBytes {
			return nil, fmt.Errorf("%w: invalid key %q", ErrInvalidKey, id)
		}
		copied[id] = append([]byte(nil), key...)
	}
	if _, ok := copied[activeID]; !ok {
		return nil, fmt.Errorf("%w: active key %q is missing", ErrInvalidKey, activeID)
	}
	return &Keyring{activeID: activeID, keys: copied, now: time.Now, options: normalizeOptions(options)}, nil
}

func (k *Keyring) Sign(claims Claims) (string, error) {
	if k == nil {
		return "", ErrInvalidKey
	}
	signer, err := NewSignerWithID(k.activeID, k.keys[k.activeID], k.options)
	if err != nil {
		return "", err
	}
	signer.now = k.now
	return signer.Sign(claims)
}

func (k *Keyring) Verify(token string) (Claims, error) {
	if k == nil {
		return Claims{}, ErrInvalidKey
	}
	parts, err := tokenParts(token)
	if err != nil {
		return Claims{}, err
	}
	key, ok := k.keys[parts[0]]
	if !ok {
		return Claims{}, ErrUnknownKey
	}
	return verifyWithKey(token, parts[0], key, k.now().UTC(), k.options)
}

func verifyWithKey(token, expectedKeyID string, key []byte, now time.Time, options Options) (Claims, error) {
	if len(key) < MinKeyBytes {
		return Claims{}, ErrInvalidKey
	}
	parts, err := tokenParts(token)
	if err != nil {
		return Claims{}, err
	}
	if parts[0] != expectedKeyID {
		return Claims{}, ErrUnknownKey
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(provided) != sha256.Size {
		return Claims{}, ErrInvalidToken
	}
	message := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.KeyID != parts[0] {
		return Claims{}, ErrInvalidToken
	}
	if err := claims.validate(now, options); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

func tokenParts(token string) ([]string, error) {
	if len(token) == 0 || len(token) > MaxTokenBytes {
		return nil, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || !validKeyID(parts[0]) || parts[1] == "" || parts[2] == "" {
		return nil, ErrInvalidToken
	}
	return parts, nil
}
