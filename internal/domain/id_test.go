package domain

import "testing"

func TestNewID(t *testing.T) {
	id, err := NewID("cam")
	if err != nil {
		t.Fatal(err)
	}
	if !id.ValidFor("cam") {
		t.Fatalf("invalid generated id: %q", id)
	}
	if id.ValidFor("dev") {
		t.Fatal("camera id validated as device id")
	}
}

func TestNewIDRejectsBadPrefix(t *testing.T) {
	for _, p := range []string{"", "bad_prefix"} {
		if _, err := NewID(p); err == nil {
			t.Fatalf("expected error for %q", p)
		}
	}
}
