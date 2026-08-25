package locks

import (
	"sync"
	"testing"
	"time"
)

func TestSerializesSameKey(t *testing.T) {
	m := New()
	release := m.Lock("camera:1")
	done := make(chan struct{})
	go func() { r := m.Lock("camera:1"); r(); close(done) }()
	select {
	case <-done:
		t.Fatal("second lock entered early")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lock did not release")
	}
	if m.ActiveKeys() != 0 {
		t.Fatalf("active=%d", m.ActiveKeys())
	}
	var _ sync.Locker
}
