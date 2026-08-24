package siren

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
	"github.com/Homiakus/axiom/adgo"
)

const DefaultMaxActivation = 2 * time.Minute

type Config struct {
	Production            adgo.ProductionConfig
	WorkerID              string
	WorkerConcurrency     int
	MaxActivationDuration time.Duration
}

func DefaultConfig(root string) Config {
	return Config{
		Production: adgo.DefaultProductionConfig(root), WorkerID: "home-sentinel-siren",
		WorkerConcurrency: 1, MaxActivationDuration: DefaultMaxActivation,
	}
}

type Service struct {
	production *adgo.Production
	worker     adgo.WorkerSpec
	startMu    sync.Mutex
}

func Open(config Config, deps Dependencies) (*Service, error) {
	maxDuration := config.MaxActivationDuration
	if maxDuration <= 0 {
		maxDuration = DefaultMaxActivation
	}
	plan, err := CompilePlan(maxDuration)
	if err != nil {
		return nil, fmt.Errorf("compile siren plan: %w", err)
	}
	production, err := adgo.OpenProduction(plan, NewRegistry(deps), config.Production)
	if err != nil {
		return nil, fmt.Errorf("open siren runtime: %w", err)
	}
	workerID := config.WorkerID
	if workerID == "" {
		workerID = "home-sentinel-siren"
	}
	concurrency := config.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Service{production: production, worker: adgo.WorkerSpec{ID: workerID, Concurrency: concurrency}}, nil
}

func (s *Service) Close() error {
	if s == nil || s.production == nil {
		return nil
	}
	return s.production.Close()
}

func (s *Service) Start(ctx context.Context, request domainaction.SirenRequest) (*adgo.Execution, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	id := domainaction.SirenExecutionID(request)
	resource := sirenResourceKey(request.SirenID)

	s.startMu.Lock()
	defer s.startMu.Unlock()
	if err := resourceguard.Check(ctx, s.production.Store, PlanID, id, resource, persistedSirenResource); err != nil {
		return nil, err
	}
	return s.production.Engine.StartOrLoad(ctx, id, map[string]any{"request": request}, adgo.BudgetLimit{})
}

func sirenResourceKey(sirenID string) string { return "siren:" + strings.TrimSpace(sirenID) }

func persistedSirenResource(execution *adgo.Execution) (string, error) {
	raw, ok := execution.Data["request"]
	if !ok {
		return "", fmt.Errorf("siren: persisted request is missing")
	}
	var request domainaction.SirenRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return "", fmt.Errorf("siren: decode persisted request: %w", err)
	}
	if strings.TrimSpace(request.SirenID) == "" {
		return "", fmt.Errorf("siren: persisted siren id is empty")
	}
	return sirenResourceKey(request.SirenID), nil
}

func (s *Service) Drive(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

// Stop requests cancellation. ADGO then runs the enable node's compensation,
// which is an idempotent ensure-disabled operation, so manual stop is fail-safe.
func (s *Service) Stop(ctx context.Context, executionID, reason string) (*adgo.Execution, error) {
	if _, err := s.production.Engine.Cancel(ctx, executionID, reason); err != nil {
		return nil, err
	}
	if _, err := s.production.Engine.Advance(ctx, executionID); err != nil {
		return nil, err
	}
	return s.production.Engine.Get(ctx, executionID)
}

func (s *Service) Get(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.Get(ctx, executionID)
}

func (s *Service) Serve(ctx context.Context) error {
	return s.production.Serve(ctx, s.worker)
}
