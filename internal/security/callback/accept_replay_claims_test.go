package callback

import (
	"errors"
	"testing"
	"time"
)

func TestAcceptorReplayReturnsVerifiedBoundClaims(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	ring, acceptor := newTestAcceptor(t, now)
	claims := boundClaims(now, "nonce-replay-claims")
	claims.Subject = "usr_11111111111111111111111111"
	token, err := ring.Sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := acceptor.Accept(token, boundExpectation()); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	got, err := acceptor.Accept(token, boundExpectation())
	if !errors.Is(err, ErrReplay) {
		t.Fatalf("second accept error=%v want ErrReplay", err)
	}
	if got.ExecutionID != claims.ExecutionID ||
		got.NodeID != claims.NodeID ||
		got.EventID != claims.EventID ||
		got.Action != claims.Action ||
		got.Subject != claims.Subject ||
		got.Nonce != claims.Nonce {
		t.Fatalf("verified replay claims changed: got=%+v want=%+v", got, claims)
	}
}
