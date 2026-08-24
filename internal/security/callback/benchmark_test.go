package callback

import (
	"strings"
	"testing"
	"time"
)

func BenchmarkVerify(b *testing.B) {
	now := time.Unix(1_700_000_000, 0).UTC()
	signer, err := NewSignerWithID("bench", []byte(strings.Repeat("k", 32)), DefaultOptions)
	if err != nil {
		b.Fatal(err)
	}
	signer.now = func() time.Time { return now }
	token, err := signer.Sign(Claims{
		ExecutionID: "incident-bench",
		NodeID:      "human-decision",
		EventID:     "event-bench",
		Nonce:       "nonce-bench",
		ExpiresAt:   now.Add(time.Minute).Unix(),
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := signer.Verify(token); err != nil {
			b.Fatal(err)
		}
	}
}
