package camera

import (
	"context"
	"fmt"

	"github.com/Homiakus/axiom"
)

// Service hides Axiom runtime details from the rest of Home Sentinel.
type Service struct {
	engine *axiom.Engine
}

func NewService() (*Service, error) {
	engine, err := axiom.Open(Definition())
	if err != nil {
		return nil, fmt.Errorf("open camera lifecycle: %w", err)
	}
	return &Service{engine: engine}, nil
}

func (s *Service) Connected(ctx context.Context, cameraID, endpoint string) error {
	return s.engine.Execution(cameraID).Dispatch(ctx, Connected{Endpoint: endpoint})
}

func (s *Service) Degraded(ctx context.Context, cameraID, reason string) error {
	return s.engine.Execution(cameraID).Dispatch(ctx, StreamDegraded{Reason: reason})
}

func (s *Service) Failed(ctx context.Context, cameraID, reason string) error {
	return s.engine.Execution(cameraID).Dispatch(ctx, StreamFailed{Reason: reason})
}

func (s *Service) Recovered(ctx context.Context, cameraID, probe string) error {
	return s.engine.Execution(cameraID).Dispatch(ctx, Recovered{Probe: probe})
}

func (s *Service) Disable(ctx context.Context, cameraID, reason string) error {
	return s.engine.Execution(cameraID).Dispatch(ctx, DisableRequested{Reason: reason})
}

func (s *Service) Enable(ctx context.Context, cameraID, reason string) error {
	return s.engine.Execution(cameraID).Dispatch(ctx, EnableRequested{Reason: reason})
}

func (s *Service) State(ctx context.Context, cameraID string) (State, error) {
	var state State
	if err := s.engine.Execution(cameraID).State(ctx, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) History(ctx context.Context, cameraID string) ([]axiom.HistoryEntry, error) {
	return s.engine.Execution(cameraID).History(ctx)
}
