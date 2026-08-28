package callback

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type recordingSecretResolver struct {
	values map[secrets.Ref][]byte
	err    error
}

func (r *recordingSecretResolver) Resolve(ref secrets.Ref) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	value, ok := r.values[ref]
	if !ok {
		return nil, errors.New("secret not found")
	}
	return value, nil
}

func TestOpenRuntimeCopiesThenWipesResolvedKeyMaterial(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	ref := secrets.Ref("secret://env/CALLBACK_ACTIVE")
	resolved := []byte(strings.Repeat("k", MinKeyBytes))
	resolver := &recordingSecretResolver{values: map[secrets.Ref][]byte{ref: resolved}}
	runtime, err := OpenRuntime(RuntimeConfig{
		ActiveKeyID:    "active",
		Keys:           map[string]secrets.Ref{"active": ref},
		Options:        DefaultOptions,
		ReplayCapacity: 32,
	}, resolver)
	if err != nil {
		t.Fatalf("OpenRuntime: %v", err)
	}
	for i, value := range resolved {
		if value != 0 {
			t.Fatalf("resolved key byte %d was not wiped", i)
		}
	}
	runtime.keyring.now = func() time.Time { return now }
	runtime.acceptor.now = func() time.Time { return now }
	claims := Claims{
		ExecutionID: "incident-1",
		NodeID:      "owner-high-risk-decision",
		EventID:     "decision-1",
		Action:      "incident.owner.decision",
		Nonce:       "nonce-1",
		ExpiresAt:   now.Add(time.Minute).Unix(),
	}
	token, err := runtime.Sign(claims)
	if err != nil {
		t.Fatalf("Sign after source wipe: %v", err)
	}
	if _, err := runtime.Accept(token, Binding{
		ExecutionID: claims.ExecutionID,
		NodeID:      claims.NodeID,
		EventID:     claims.EventID,
		Action:      claims.Action,
	}); err != nil {
		t.Fatalf("Accept after source wipe: %v", err)
	}
}

func TestOpenRuntimeSupportsReviewedKeyRotationOverlap(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	oldRef := secrets.Ref("secret://env/CALLBACK_OLD")
	newRef := secrets.Ref("secret://env/CALLBACK_NEW")
	oldKey := []byte(strings.Repeat("o", MinKeyBytes))
	newKey := []byte(strings.Repeat("n", MinKeyBytes))
	resolver := &recordingSecretResolver{values: map[secrets.Ref][]byte{
		oldRef: append([]byte(nil), oldKey...),
		newRef: append([]byte(nil), newKey...),
	}}
	runtime, err := OpenRuntime(RuntimeConfig{
		ActiveKeyID: "new",
		Keys: map[string]secrets.Ref{
			"old": oldRef,
			"new": newRef,
		},
		Options:        DefaultOptions,
		ReplayCapacity: 32,
	}, resolver)
	if err != nil {
		t.Fatalf("OpenRuntime: %v", err)
	}
	runtime.keyring.now = func() time.Time { return now }
	runtime.acceptor.now = func() time.Time { return now }

	claims := Claims{
		ExecutionID: "incident-rotation",
		NodeID:      "await-owner-ack",
		EventID:     "event-old",
		Action:      "incident.owner.response",
		Nonce:       "old-nonce",
		ExpiresAt:   now.Add(time.Minute).Unix(),
	}
	oldSigner, err := NewSignerWithID("old", oldKey, DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	oldSigner.now = func() time.Time { return now }
	oldToken, err := oldSigner.Sign(claims)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Accept(oldToken, Binding{
		ExecutionID: claims.ExecutionID,
		NodeID:      claims.NodeID,
		EventID:     claims.EventID,
		Action:      claims.Action,
	}); err != nil {
		t.Fatalf("old reviewed key rejected during overlap: %v", err)
	}

	newClaims := claims
	newClaims.EventID = "event-new"
	newClaims.Nonce = "new-nonce"
	newToken, err := runtime.Sign(newClaims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := runtime.keyring.Verify(newToken)
	if err != nil {
		t.Fatalf("active key token rejected: %v", err)
	}
	if verified.KeyID != "new" {
		t.Fatalf("Sign used key %q, want new", verified.KeyID)
	}
}

func TestOpenRuntimeFailsClosedOnResolverAndReplayPolicy(t *testing.T) {
	ref := secrets.Ref("secret://env/CALLBACK_ACTIVE")
	resolver := &recordingSecretResolver{err: errors.New("provider unavailable")}
	_, err := OpenRuntime(RuntimeConfig{
		ActiveKeyID:    "active",
		Keys:           map[string]secrets.Ref{"active": ref},
		Options:        DefaultOptions,
		ReplayCapacity: 32,
	}, resolver)
	if err == nil || !strings.Contains(err.Error(), "resolve key") {
		t.Fatalf("resolver failure did not fail closed: %v", err)
	}

	resolver = &recordingSecretResolver{values: map[secrets.Ref][]byte{
		ref: []byte(strings.Repeat("a", MinKeyBytes)),
	}}
	_, err = OpenRuntime(RuntimeConfig{
		ActiveKeyID:    "active",
		Keys:           map[string]secrets.Ref{"active": ref},
		Options:        DefaultOptions,
		ReplayCapacity: 0,
	}, resolver)
	if err == nil {
		t.Fatal("zero replay capacity accepted")
	}
}
