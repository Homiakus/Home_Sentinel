package callback

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestAcceptor(t *testing.T, now time.Time) (*Keyring, *Acceptor) {
	t.Helper()
	key := []byte(strings.Repeat("a", MinKeyBytes))
	ring, err := NewKeyring("active", map[string][]byte{"active": key}, DefaultOptions)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	ring.now = func() time.Time { return now }
	guard, err := NewReplayGuard(128)
	if err != nil {
		t.Fatalf("new replay guard: %v", err)
	}
	acceptor, err := NewAcceptor(ring, guard)
	if err != nil {
		t.Fatalf("new acceptor: %v", err)
	}
	acceptor.now = func() time.Time { return now }
	return ring, acceptor
}

func boundClaims(now time.Time, nonce string) Claims {
	return Claims{
		ExecutionID: "incident-42",
		NodeID:      "human-decision",
		EventID:     "decision-event-7",
		Action:      "incident.owner-decision",
		Nonce:       nonce,
		ExpiresAt:   now.Add(2 * time.Minute).Unix(),
	}
}

func boundExpectation() Binding {
	return Binding{
		ExecutionID: "incident-42",
		NodeID:      "human-decision",
		EventID:     "decision-event-7",
		Action:      "incident.owner-decision",
	}
}

func TestAcceptorRequiresExactBindingAndDoesNotBurnMismatchedToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	fields := []struct {
		name   string
		mutate func(*Binding)
	}{
		{name: "execution", mutate: func(b *Binding) { b.ExecutionID = "incident-other" }},
		{name: "node", mutate: func(b *Binding) { b.NodeID = "other-node" }},
		{name: "event", mutate: func(b *Binding) { b.EventID = "other-event" }},
		{name: "action", mutate: func(b *Binding) { b.Action = "incident.other-action" }},
	}

	for i, tc := range fields {
		t.Run(tc.name, func(t *testing.T) {
			ring, acceptor := newTestAcceptor(t, now)
			claims := boundClaims(now, "nonce-mismatch-"+string(rune('a'+i)))
			token, err := ring.Sign(claims)
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			wrong := boundExpectation()
			tc.mutate(&wrong)
			if _, err := acceptor.Accept(token, wrong); !errors.Is(err, ErrBindingMismatch) {
				t.Fatalf("mismatch accepted: %v", err)
			}
			if _, err := acceptor.Accept(token, boundExpectation()); err != nil {
				t.Fatalf("mismatched route burned otherwise valid token: %v", err)
			}
			if _, err := acceptor.Accept(token, boundExpectation()); !errors.Is(err, ErrReplay) {
				t.Fatalf("second correct delivery was not rejected as replay: %v", err)
			}
		})
	}
}

func TestAcceptorRejectsMissingActionBinding(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	ring, acceptor := newTestAcceptor(t, now)
	claims := boundClaims(now, "nonce-no-action")
	claims.Action = ""
	token, err := ring.Sign(claims)
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}
	if _, err := acceptor.Accept(token, boundExpectation()); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("legacy token without action reached Stage-17 ingress: %v", err)
	}

	expected := boundExpectation()
	expected.Action = ""
	if _, err := acceptor.Accept(token, expected); !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("empty expected action accepted: %v", err)
	}
}

func TestAcceptorRejectsTamperingBeforeReplayAdmission(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	ring, acceptor := newTestAcceptor(t, now)
	token, err := ring.Sign(boundClaims(now, "nonce-tamper"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + parts[1] + "A." + parts[2]
	if _, err := acceptor.Accept(tampered, boundExpectation()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token accepted: %v", err)
	}
	if _, err := acceptor.Accept(token, boundExpectation()); err != nil {
		t.Fatalf("tampered attempt burned valid token: %v", err)
	}
}

func TestAcceptorConcurrentReplayHasExactlyOneWinner(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	ring, acceptor := newTestAcceptor(t, now)
	token, err := ring.Sign(boundClaims(now, "nonce-concurrent"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	const workers = 32
	start := make(chan struct{})
	results := make(chan error, workers)
	var ready sync.WaitGroup
	ready.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			ready.Done()
			<-start
			_, err := acceptor.Accept(token, boundExpectation())
			results <- err
		}()
	}
	ready.Wait()
	close(start)

	var accepted atomic.Int32
	var replayed atomic.Int32
	for i := 0; i < workers; i++ {
		err := <-results
		switch {
		case err == nil:
			accepted.Add(1)
		case errors.Is(err, ErrReplay):
			replayed.Add(1)
		default:
			t.Fatalf("unexpected concurrent acceptance error: %v", err)
		}
	}
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted=%d want=1", got)
	}
	if got := replayed.Load(); got != workers-1 {
		t.Fatalf("replayed=%d want=%d", got, workers-1)
	}
}
