package callback

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func testSigner(t *testing.T, now time.Time) *Signer {
	t.Helper()
	signer, err := NewSignerWithID("k1", []byte(strings.Repeat("k", 32)), DefaultOptions)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	signer.now = func() time.Time { return now }
	return signer
}

func validClaims(now time.Time) Claims {
	return Claims{
		ExecutionID: "incident-1",
		NodeID:      "human",
		EventID:     "event-1",
		Nonce:       "nonce-1",
		ExpiresAt:   now.Add(time.Minute).Unix(),
	}
}

func TestTokenRoundTripAndTamperDetection(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	signer := testSigner(t, now)
	token, err := signer.Sign(validClaims(now))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	verified, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified.KeyID != "k1" || verified.IssuedAt != now.Unix() || verified.ExecutionID != "incident-1" {
		t.Fatalf("unexpected claims: %#v", verified)
	}

	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + parts[1] + "A." + parts[2]
	if _, err := signer.Verify(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token accepted: %v", err)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	signer := testSigner(t, now)
	claims := validClaims(now)
	claims.ExpiresAt = now.Add(time.Second).Unix()
	token, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	signer.now = func() time.Time { return now.Add(DefaultOptions.ClockSkew + 2*time.Second) }
	if _, err := signer.Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired token accepted: %v", err)
	}
}

func TestTTLAndFutureIssueTimeRejected(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	signer := testSigner(t, now)
	claims := validClaims(now)
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(DefaultOptions.MaxTTL + time.Second).Unix()
	if _, err := signer.Sign(claims); !errors.Is(err, ErrTTLExceeded) {
		t.Fatalf("oversized ttl accepted: %v", err)
	}

	claims = validClaims(now)
	claims.IssuedAt = now.Add(DefaultOptions.ClockSkew + time.Second).Unix()
	claims.ExpiresAt = time.Unix(claims.IssuedAt, 0).Add(time.Minute).Unix()
	if _, err := signer.Sign(claims); !errors.Is(err, ErrNotYetValid) {
		t.Fatalf("future-issued token accepted: %v", err)
	}
}

func TestKeyringSupportsRotationOverlap(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	oldKey := []byte(strings.Repeat("o", 32))
	newKey := []byte(strings.Repeat("n", 32))
	oldSigner, err := NewSignerWithID("old", oldKey, DefaultOptions)
	if err != nil {
		t.Fatalf("old signer: %v", err)
	}
	oldSigner.now = func() time.Time { return now }
	oldToken, err := oldSigner.Sign(validClaims(now))
	if err != nil {
		t.Fatalf("sign old token: %v", err)
	}

	ring, err := NewKeyring("new", map[string][]byte{"old": oldKey, "new": newKey}, DefaultOptions)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	ring.now = func() time.Time { return now }
	if _, err := ring.Verify(oldToken); err != nil {
		t.Fatalf("rotation overlap rejected old token: %v", err)
	}
	newToken, err := ring.Sign(validClaims(now))
	if err != nil {
		t.Fatalf("sign active token: %v", err)
	}
	verified, err := ring.Verify(newToken)
	if err != nil || verified.KeyID != "new" {
		t.Fatalf("active key token failed: %#v, %v", verified, err)
	}

	retired, err := NewKeyring("new", map[string][]byte{"new": newKey}, DefaultOptions)
	if err != nil {
		t.Fatalf("retired ring: %v", err)
	}
	retired.now = func() time.Time { return now }
	if _, err := retired.Verify(oldToken); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("retired key still accepted: %v", err)
	}
}

func TestReplayGuardRejectsSecondUse(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	guard, err := NewReplayGuard(4)
	if err != nil {
		t.Fatalf("new replay guard: %v", err)
	}
	claims := validClaims(now)
	claims.KeyID = "k1"
	claims.IssuedAt = now.Unix()
	if err := guard.Consume(claims, now); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if err := guard.Consume(claims, now); !errors.Is(err, ErrReplay) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestShortKeyAndOversizedTokenRejected(t *testing.T) {
	if _, err := NewSigner([]byte("short")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
	signer := testSigner(t, time.Unix(1_700_000_000, 0).UTC())
	if _, err := signer.Verify(strings.Repeat("x", MaxTokenBytes+1)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("oversized token accepted: %v", err)
	}
}
