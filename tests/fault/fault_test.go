package fault_test

import (
	"context"
	"errors"
	"github.com/Homiakus/Home_Sentinel/internal/health"
	"github.com/Homiakus/Home_Sentinel/internal/watchdog"
	"sync/atomic"
	"testing"
	"time"
)

func TestBrokerFailureIsRootCauseForHA(t *testing.T) {
	r := health.NewRegistry()
	r.Set("mqtt", health.Failed, "BROKER_DOWN", "dial")
	r.Set("home_assistant", health.Degraded, "MQTT_UNAVAILABLE", "discovery")
	d := health.Diagnose(r, health.DependencyGraph{"home_assistant": {"mqtt"}})
	for _, x := range d {
		if x.Component.Name == "home_assistant" && x.SuppressedBy != "mqtt" {
			t.Fatalf("bad diagnosis %+v", x)
		}
	}
}
func TestOptionalAIFailureDoesNotChangeCoreHealth(t *testing.T) {
	r := health.NewRegistry()
	r.Set("sentinel", health.Healthy, "", "")
	r.Set("frigate", health.Healthy, "", "")
	r.Set("ai", health.Failed, "MODEL_DOWN", "timeout")
	s, _ := r.Get("sentinel")
	f, _ := r.Get("frigate")
	if s.Status != health.Healthy || f.Status != health.Healthy {
		t.Fatal("optional AI failure contaminated critical components")
	}
}
func TestWatchdogBackoffBoundsFlappingProbe(t *testing.T) {
	reg := health.NewRegistry()
	var n atomic.Int32
	m := &watchdog.Manager{Registry: reg, Checks: []watchdog.Check{{Name: "flap", Base: 10 * time.Millisecond, MaxBackoff: 40 * time.Millisecond, Timeout: 5 * time.Millisecond, Probe: func(context.Context) error { n.Add(1); return errors.New("down") }}}}
	ctx, cancel := context.WithCancel(context.Background())
	if e := m.Start(ctx); e != nil {
		t.Fatal(e)
	}
	time.Sleep(115 * time.Millisecond)
	cancel()
	m.Close()
	if calls := n.Load(); calls > 7 {
		t.Fatalf("restart/probe storm: %d calls", calls)
	}
	c, _ := reg.Get("flap")
	if c.Status != health.Failed {
		t.Fatalf("expected failed %+v", c)
	}
}
