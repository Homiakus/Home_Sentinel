package redact

import (
	"strings"
	"testing"
)

func TestStringRedactsURLAndKV(t *testing.T) {
	in := "probe rtsp://admin:hunter2@10.0.0.2/live?token=abc password=supersecret"
	got := String(in)
	for _, secret := range []string{"hunter2", "abc", "supersecret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
}
