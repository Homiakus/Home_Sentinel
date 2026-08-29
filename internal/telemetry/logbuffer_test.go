package telemetry

import (
	"strings"
	"testing"
)

func TestLogBufferRingAndRedaction(t *testing.T) {
	buf := NewLogBuffer(3, 2048)
	_, _ = buf.Write([]byte("one"))
	if got := buf.Snapshot(0); len(got.Lines) != 0 {
		t.Fatalf("unterminated line retained early: %#v", got.Lines)
	}
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte(`{"level":"INFO","msg":"connected","token":"top-secret-token","nested":{"password":"top-secret-password"}}` + "\n"))
	_, _ = buf.Write([]byte("Authorization: Bearer abc.def.ghi\n"))
	_, _ = buf.Write([]byte("four\n"))

	snap := buf.Snapshot(0)
	if snap.Retained != 3 || snap.Capacity != 3 {
		t.Fatalf("unexpected snapshot metadata: %+v", snap)
	}
	joined := strings.Join(snap.Lines, "\n")
	if strings.Contains(joined, "top-secret-token") || strings.Contains(joined, "top-secret-password") || strings.Contains(joined, "abc.def.ghi") {
		t.Fatalf("sensitive value leaked: %s", joined)
	}
	if !strings.Contains(joined, "[REDACTED]") {
		t.Fatalf("expected redaction marker: %s", joined)
	}
	if snap.Lines[len(snap.Lines)-1] != "four" {
		t.Fatalf("ring order lost: %#v", snap.Lines)
	}

	limited := buf.Snapshot(2)
	if len(limited.Lines) != 2 || limited.Lines[1] != "four" {
		t.Fatalf("limit did not return newest lines: %#v", limited.Lines)
	}
}

func TestLogBufferBoundsUnterminatedInput(t *testing.T) {
	buf := NewLogBuffer(2, 16)
	_, _ = buf.Write([]byte(strings.Repeat("x", 40)))
	snap := buf.Snapshot(0)
	if len(snap.Lines) != 1 || !strings.Contains(snap.Lines[0], "[truncated]") {
		t.Fatalf("expected bounded truncated line, got %#v", snap.Lines)
	}
}
