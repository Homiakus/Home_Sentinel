package backup

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Schedule struct {
	Interval time.Duration `json:"interval"`
}
type Scheduler struct {
	Manager  *Manager
	Schedule Schedule
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func (s *Scheduler) Start(parent context.Context) error {
	if s.Manager == nil {
		return errors.New("backup scheduler manager required")
	}
	if s.Schedule.Interval <= 0 {
		return errors.New("backup schedule interval must be positive")
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.Schedule.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run, stop := context.WithTimeout(ctx, minDuration(s.Schedule.Interval/2, 2*time.Hour))
				_, _ = s.Manager.RunCritical(run)
				stop()
			}
		}
	}()
	return nil
}
func (s *Scheduler) Close() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
	s.cancel = nil
}
func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if a < b {
		return a
	}
	return b
}
