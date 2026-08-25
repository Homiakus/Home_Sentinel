package simulator

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Advance(d time.Duration) time.Time
	Set(t time.Time)
}

type VirtualClock struct {
	mu      sync.RWMutex
	current time.Time
}

func NewVirtualClock(start time.Time) *VirtualClock {
	if start.IsZero() {
		start = time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	}
	return &VirtualClock{current: start.UTC()}
}

func (c *VirtualClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

func (c *VirtualClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.current.Add(d)
	return c.current
}

func (c *VirtualClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = t.UTC()
}
