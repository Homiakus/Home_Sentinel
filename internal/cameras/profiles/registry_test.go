package profiles

import "testing"

func TestDefaultRegistry(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.All()) < 4 {
		t.Fatalf("profiles=%d", len(r.All()))
	}
	if p := r.Match("HIKVISION DS-2CD"); p.ID != "hikvision" {
		t.Fatalf("match=%s", p.ID)
	}
	if p := r.Match("Unknown Camera"); p.ID != "generic" {
		t.Fatalf("fallback=%s", p.ID)
	}
}
