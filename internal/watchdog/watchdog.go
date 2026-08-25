package watchdog

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/health"
)

type Probe func(context.Context) error

type Check struct {
	Name       string
	Probe      Probe
	Base       time.Duration
	MaxBackoff time.Duration
	Timeout    time.Duration
	ReasonCode string
}

type Manager struct {
	Registry *health.Registry
	Checks   []Check
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func (m *Manager) Start(parent context.Context) error {
	if m == nil || m.Registry == nil {
		return errors.New("watchdog registry required")
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	for _, c := range m.Checks {
		if c.Name == "" || c.Probe == nil {
			cancel()
			return errors.New("watchdog check name/probe required")
		}
		if c.Base <= 0 {
			c.Base = 30 * time.Second
		}
		if c.MaxBackoff < c.Base {
			c.MaxBackoff = 5 * time.Minute
		}
		if c.Timeout <= 0 {
			c.Timeout = 5 * time.Second
		}
		check := c
		m.wg.Add(1)
		go m.run(ctx, check)
	}
	return nil
}

func (m *Manager) run(ctx context.Context, c Check) {
	defer m.wg.Done()
	delay := time.Duration(0)
	failures := 0
	for {
		if delay > 0 {
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
		probeCtx, cancel := context.WithTimeout(ctx, c.Timeout)
		err := c.Probe(probeCtx)
		cancel()
		if err == nil {
			failures = 0
			delay = c.Base
			m.Registry.Set(c.Name, health.Healthy, "", "")
			continue
		}
		failures++
		status := health.Degraded
		if failures >= 3 {
			status = health.Failed
		}
		reason := c.ReasonCode
		if reason == "" {
			reason = "PROBE_FAILED"
		}
		m.Registry.Set(c.Name, status, reason, "health probe failed")
		if delay <= 0 {
			delay = c.Base
		} else {
			delay *= 2
		}
		if delay > c.MaxBackoff {
			delay = c.MaxBackoff
		}
	}
}

func (m *Manager) Close() {
	if m == nil || m.cancel == nil {
		return
	}
	m.cancel()
	m.wg.Wait()
	m.cancel = nil
}
