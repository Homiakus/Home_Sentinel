package search

import "testing"

func TestMatchScore(t *testing.T) {
	if got := matchScore("front", "cam_1 front door hikvision", "front door"); got < 30 {
		t.Fatalf("unexpected score %d", got)
	}
	if got := matchScore("missing", "front door", "front door"); got != 0 {
		t.Fatalf("expected no match, got %d", got)
	}
}
