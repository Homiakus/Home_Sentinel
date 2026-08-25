package incidents

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/events"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type Service struct {
	Events    *repository.Store[events.Envelope]
	Incidents *repository.Store[Incident]
	Engine    *Engine
	Bus       *events.Bus

	mu     sync.RWMutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewService(bus *events.Bus, eventStore *repository.Store[events.Envelope], incidentStore *repository.Store[Incident]) *Service {
	return &Service{Bus: bus, Events: eventStore, Incidents: incidentStore, Engine: NewEngine(45 * time.Second)}
}

func (s *Service) Start(parent context.Context) error {
	if s == nil || s.Bus == nil || s.Events == nil || s.Incidents == nil || s.Engine == nil {
		return errors.New("incident service dependencies unavailable")
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	sub := s.Bus.Subscribe(1024)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer sub.Cancel()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.C:
				if !ok {
					return
				}
				_ = s.Ingest(ctx, ev)
			case now := <-ticker.C:
				s.closeStale(ctx, now.UTC())
			}
		}
	}()
	return nil
}

func (s *Service) Close() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
	s.cancel = nil
}

func (s *Service) Ingest(ctx context.Context, ev events.Envelope) error {
	if err := ev.Validate(); err != nil {
		return err
	}
	if _, err := s.Events.Put(ctx, ev.ID.String(), ev); err != nil {
		return err
	}
	s.mu.Lock()
	inc, err := s.Engine.Process(ev)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	_, err = s.Incidents.Put(ctx, inc.ID.String(), inc)
	return err
}

func (s *Service) closeStale(ctx context.Context, now time.Time) {
	s.mu.Lock()
	items := s.Engine.CloseStale(now)
	s.mu.Unlock()
	for _, inc := range items {
		_, _ = s.Incidents.Put(ctx, inc.ID.String(), inc)
	}
}

func (s *Service) List(ctx context.Context, limit int) ([]repository.Resource[Incident], error) {
	return s.Incidents.List(ctx, limit)
}

func (s *Service) Get(ctx context.Context, id string) (repository.Resource[Incident], error) {
	return s.Incidents.Get(ctx, id)
}

func (s *Service) Event(ctx context.Context, id string) (repository.Resource[events.Envelope], error) {
	return s.Events.Get(ctx, id)
}
