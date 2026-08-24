package incident

import (
	"context"
	"encoding/json"
	"fmt"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/axiom/adgo"
)

type Config struct {
	Production        adgo.ProductionConfig
	WorkerID          string
	WorkerConcurrency int
}

func DefaultConfig(root string) Config {
	return Config{
		Production:        adgo.DefaultProductionConfig(root),
		WorkerID:          "home-sentinel-local",
		WorkerConcurrency: 4,
	}
}

type Service struct {
	production *adgo.Production
	worker     adgo.WorkerSpec
}

func Open(config Config, deps Dependencies) (*Service, error) {
	plan, err := CompilePlan()
	if err != nil {
		return nil, fmt.Errorf("compile incident plan: %w", err)
	}
	production, err := adgo.OpenProduction(plan, NewRegistry(deps), config.Production)
	if err != nil {
		return nil, fmt.Errorf("open incident production runtime: %w", err)
	}
	concurrency := config.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	workerID := config.WorkerID
	if workerID == "" {
		workerID = "home-sentinel-local"
	}
	return &Service{
		production: production,
		worker:     adgo.WorkerSpec{ID: workerID, Concurrency: concurrency},
	}, nil
}

func (s *Service) Close() error {
	if s == nil || s.production == nil {
		return nil
	}
	return s.production.Close()
}

func (s *Service) Start(ctx context.Context, trigger domainincident.Trigger) (*adgo.Execution, error) {
	if err := trigger.Validate(); err != nil {
		return nil, err
	}
	id := domainincident.ExecutionID(trigger)
	return s.production.Engine.StartOrLoad(ctx, id, map[string]any{"trigger": trigger}, adgo.BudgetLimit{})
}

// Drive executes currently runnable work for one incident and returns when it
// is terminal or durably waiting for an external event.
func (s *Service) Drive(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) OwnerResponse(ctx context.Context, executionID, eventID string, payload any) (*adgo.Execution, error) {
	var raw json.RawMessage
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode owner response: %w", err)
		}
		raw = encoded
	}
	if err := s.production.Engine.Signal(ctx, executionID, adgo.Event{
		ID: eventID, Type: OwnerResponseEvent, TargetNode: NodeAwait, Payload: raw,
	}); err != nil {
		return nil, err
	}
	return s.Drive(ctx, executionID)
}

func (s *Service) Get(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.Get(ctx, executionID)
}

// Serve runs the resilient coordinator, schedule runner and worker loop. It is
// intended for the long-lived sentinel process; Drive remains useful for tests
// and request-scoped embedded execution.
func (s *Service) Serve(ctx context.Context) error {
	return s.production.Serve(ctx, s.worker)
}
