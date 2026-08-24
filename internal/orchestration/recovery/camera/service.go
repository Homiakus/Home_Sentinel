package camera

import (
	"context"
	"fmt"

	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	"github.com/Homiakus/axiom/adgo"
)

type Config struct {
	Production        adgo.ProductionConfig
	WorkerID          string
	WorkerConcurrency int
}

func DefaultConfig(root string) Config {
	return Config{Production: adgo.DefaultProductionConfig(root), WorkerID: "home-sentinel-camera-recovery", WorkerConcurrency: 1}
}

type Service struct {
	production *adgo.Production
	worker     adgo.WorkerSpec
}

func Open(config Config, deps Dependencies) (*Service, error) {
	plan, err := CompilePlan()
	if err != nil {
		return nil, fmt.Errorf("compile camera recovery plan: %w", err)
	}
	production, err := adgo.OpenProduction(plan, NewRegistry(deps), config.Production)
	if err != nil {
		return nil, fmt.Errorf("open camera recovery runtime: %w", err)
	}
	workerID := config.WorkerID
	if workerID == "" {
		workerID = "home-sentinel-camera-recovery"
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

func (s *Service) Start(ctx context.Context, request domainrecovery.CameraRequest) (*adgo.Execution, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return s.production.Engine.StartOrLoad(
		ctx, domainrecovery.CameraExecutionID(request), map[string]any{"request": request}, adgo.BudgetLimit{},
	)
}

func (s *Service) Drive(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) ResolveOperator(
	ctx context.Context,
	executionID string,
	decision domainrecovery.OperatorDecision,
	actor, reason string,
) (*adgo.Execution, error) {
	var mapped adgo.HumanDecision
	switch decision {
	case domainrecovery.OperatorRetry:
		// A NodeHuman approval follows the explicit second-attempt branch.
		mapped = adgo.HumanApprove
	case domainrecovery.OperatorReject:
		mapped = adgo.HumanReject
	case domainrecovery.OperatorAbort:
		mapped = adgo.HumanAbort
	default:
		return nil, fmt.Errorf("camera recovery: unsupported operator decision %q", decision)
	}
	if _, err := s.production.Engine.ResolveHuman(ctx, executionID, NodeOperator, adgo.HumanResolution{
		Decision: mapped, Actor: actor, Reason: reason,
	}); err != nil {
		return nil, err
	}
	return s.Drive(ctx, executionID)
}

func (s *Service) Get(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.Get(ctx, executionID)
}

func (s *Service) Serve(ctx context.Context) error {
	return s.production.Serve(ctx, s.worker)
}
