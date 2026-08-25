package httpserver

import (
	"testing"
	"time"
)

func TestRateLimiterWindow(t *testing.T) {
	now := time.Unix(100, 0)
	l := newRateLimiter()
	l.now = func() time.Time { return now }
	if !l.allow("x", 2, time.Minute) || !l.allow("x", 2, time.Minute) {
		t.Fatal("first two should pass")
	}
	if l.allow("x", 2, time.Minute) {
		t.Fatal("third should be rejected")
	}
	now = now.Add(time.Minute)
	if !l.allow("x", 2, time.Minute) {
		t.Fatal("new window should pass")
	}
}
