package door

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
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

	// startMu makes resourceguard.Check + StartOrLoad one process-local critical
	// section. The durable execution itself is the reservation across restarts.
	// Stage 24 still requires a single-writer process lock (or true distributed
	// fencing) before multi-process control plane is supported.
	startMu sync.Mutex
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
	id := domainaction.DoorExecutionID(request)
	resource := doorResourceKey(request.DoorID)

	s.startMu.Lock()
	defer s.startMu.Unlock()
	if err := resourceguard.Check(ctx, s.production.Store, PlanID, id, resource, persistedDoorResource); err != nil {
		return nil, err
	}
	return s.production.Engine.StartOrLoad(ctx, id, map[string]any{"request": request}, adgo.BudgetLimit{})
}

func doorResourceKey(doorID string) string { return "door:" + strings.TrimSpace(doorID) }

func persistedDoorResource(execution *adgo.Execution) (string, error) {
	raw, ok := execution.Data["request"]
	if !ok {
		return "", fmt.Errorf("door action: persisted request is missing")
	}
	var request domainaction.DoorRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return "", fmt.Errorf("door action: decode persisted request: %w", err)
	}
	if strings.TrimSpace(request.DoorID) == "" {
		return "", fmt.Errorf("door action: persisted door id is empty")
	}
	return doorResourceKey(request.DoorID), nil
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
