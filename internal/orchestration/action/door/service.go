package door

import (
	"context"
	"fmt"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/axiom/adgo"
)

type Config struct {
	Production        adgo.ProductionConfig
	WorkerID          string
	WorkerConcurrency int
}

func DefaultConfig(root string) Config {
	return Config{Production: adgo.DefaultProductionConfig(root), WorkerID: "home-sentinel-door", WorkerConcurrency: 1}
}

type Service struct {
	production *adgo.Production
	worker     adgo.WorkerSpec
}

func Open(config Config, deps Dependencies) (*Service, error) {
	plan, err := CompilePlan()
	if err != nil {
		return nil, fmt.Errorf("compile door plan: %w", err)
	}
	production, err := adgo.OpenProduction(plan, NewRegistry(deps), config.Production)
	if err != nil {
		return nil, fmt.Errorf("open door runtime: %w", err)
	}
	workerID := config.WorkerID
	if workerID == "" {
		workerID = "home-sentinel-door"
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

func (s *Service) Start(ctx context.Context, request domainaction.DoorRequest) (*adgo.Execution, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return s.production.Engine.StartOrLoad(
		ctx, domainaction.DoorExecutionID(request), map[string]any{"request": request}, adgo.BudgetLimit{},
	)
}

func (s *Service) Drive(ctx context.Context, executionID string) (*adgo.Execution, error) {
	return s.production.Engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) ResolveUnlockApproval(
	ctx context.Context,
	executionID string,
	decision domainaction.ApprovalDecision,
	actor, reason string,
) (*adgo.Execution, error) {
	var mapped adgo.HumanDecision
	switch decision {
	case domainaction.ApprovalApprove:
		mapped = adgo.HumanApprove
	case domainaction.ApprovalReject:
		mapped = adgo.HumanReject
	case domainaction.ApprovalAbort:
		mapped = adgo.HumanAbort
	default:
		return nil, fmt.Errorf("door action: unsupported approval %q", decision)
	}
	if _, err := s.production.Engine.ResolveHuman(ctx, executionID, NodeApproveUnlock, adgo.HumanResolution{
		Decision: mapped, Actor: actor, Reason: reason,
	}); err != nil {
		return nil, err
	}
	return s.Drive(ctx, executionID)
}

func (s *Service) ResolveReconciliation(
	ctx context.Context,
	executionID, nodeID string,
	decision domainaction.ReconcileDecision,
	actor, reason string,
) (*adgo.Execution, error) {
	if nodeID != NodeApplyLock && nodeID != NodeApplyUnlock {
		return nil, fmt.Errorf("door action: node %q is not reconcilable", nodeID)
	}
	var mapped adgo.HumanDecision
	switch decision {
	case domainaction.ReconcileConfirm:
		mapped = adgo.HumanConfirm
	case domainaction.ReconcileRetry:
		mapped = adgo.HumanRetry
	case domainaction.ReconcileAbort:
		mapped = adgo.HumanAbort
	default:
		return nil, fmt.Errorf("door action: unsupported reconciliation %q", decision)
	}
	if _, err := s.production.Engine.ResolveHuman(ctx, executionID, nodeID, adgo.HumanResolution{
		Decision: mapped, Actor: actor, Reason: reason,
	}); err != nil {
		return nil, err
	}
	return s.Drive(ctx, executionID)
}
