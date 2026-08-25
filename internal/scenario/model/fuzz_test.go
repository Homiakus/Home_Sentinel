package model

import "testing"

func FuzzDecodeNormalize(f *testing.F) {
	f.Add([]byte(`{"id":"scenario-a"}`))
	f.Add([]byte(`{"id":"scenario-a","revisionId":"rev-a","version":0,"name":"A","enabled":false,"triggers":[{"id":"trigger-a","kind":"manual","capability":{"id":"core.manual","version":"1.0.0"}}],"condition":{},"flow":{"steps":[{"id":"stop-a","kind":"stop","stop":{"outcome":"completed"}}]},"metadata":{}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		scenario, err := DecodeScenario(raw)
		if err != nil {
			return
		}
		if _, err := Normalize(scenario); err != nil {
			t.Fatalf("decoded scenario failed normalization: %v", err)
		}
		if _, err := SemanticDigest(scenario); err != nil {
			t.Fatalf("decoded scenario failed digest: %v", err)
		}
		if _, err := CanonicalJSON(scenario); err != nil {
			t.Fatalf("decoded scenario failed canonical encoding: %v", err)
		}
	})
}
