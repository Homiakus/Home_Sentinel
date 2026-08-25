package watchdog

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/health"
)

func TestWatchdogRecoversAfterFailures(t *testing.T) {
	reg := health.NewRegistry()
	var n atomic.Int32
	m := &Manager{Registry: reg, Checks: []Check{{Name: "x", Base: 5 * time.Millisecond, MaxBackoff: 10 * time.Millisecond, Timeout: time.Millisecond, Probe: func(context.Context) error {
		if n.Add(1) <= 2 {
			return errors.New("down")
		}
		return nil
	}}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, ok := reg.Get("x"); ok && c.Status == health.Healthy && n.Load() >= 3 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("did not recover, count=%d snapshot=%+v", n.Load(), reg.Snapshot())
}
