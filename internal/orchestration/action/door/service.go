package door

import (
	"context"
	"encoding/json"
	"errors"
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
	host       *adgo.Host
	bundles    *bundleCatalog
	worker     adgo.WorkerSpec

	// startMu makes resourceguard.Check + StartOrLoad one process-local critical
	// section. The durable execution itself is the reservation across restarts.
	// Stage 24 still requires a single-writer process lock (or true distributed
	// fencing) before multi-process control plane is supported.
	startMu sync.Mutex
}

func Open(config Config, deps Dependencies) (*Service, error) {
	active, err := doorV1BundleSpec(deps)
	if err != nil {
		return nil, err
	}
	return openWithBundleSpecs(config, active, []bundleSpec{active})
}

// openWithBundleSpecs is the internal release-boundary seam used to prove that
// retained v1 and a distinct future active identity can coexist before such a
// semantic v2 is ever shipped in production.
func openWithBundleSpecs(config Config, active bundleSpec, specs []bundleSpec) (*Service, error) {
	if active.plan == nil || active.registry == nil {
		return nil, errors.New("door action: active execution bundle is incomplete")
	}
	production, err := adgo.OpenProduction(active.plan, active.registry, config.Production)
	if err != nil {
		return nil, fmt.Errorf("open door runtime: %w", err)
	}
	fail := func(err error) (*Service, error) {
		_ = production.Close()
		return nil, err
	}
	host, err := adgo.NewHost(production.Store)
	if err != nil {
		return fail(fmt.Errorf("open door multi-plan host: %w", err))
	}
	engineOptions := []adgo.EngineOption{
		adgo.WithEngineLeaseTTL(config.Production.LeaseTTL),
		adgo.WithEnginePollInterval(config.Production.PollInterval),
		adgo.WithCoordinatorInterval(config.Production.CoordinatorInterval),
		adgo.WithMaxLeaseRecoveries(config.Production.MaxLeaseRecoveries),
		adgo.WithAdaptiveRouter(production.Router),
	}
	bundles, err := newBundleCatalog(host, engineOptions, active.plan.Digest, specs...)
	if err != nil {
		return fail(err)
	}
	workerID := strings.TrimSpace(config.WorkerID)
	if workerID == "" {
		workerID = "home-sentinel-door"
	}
	concurrency := config.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	service := &Service{
		production: production,
		host:       host,
		bundles:    bundles,
		worker:     adgo.WorkerSpec{ID: workerID, Concurrency: concurrency},
	}
	if err := service.validatePersistedExecutions(context.Background()); err != nil {
		return fail(err)
	}
	return service, nil
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
	if s == nil || s.production == nil || s.bundles == nil || s.bundles.active == nil || s.bundles.active.engine == nil {
		return nil, errors.New("door action: service is not open")
	}
	id := domainaction.DoorExecutionID(request)
	resource := doorResourceKey(request.DoorID)

	s.startMu.Lock()
	defer s.startMu.Unlock()
	if err := resourceguard.Check(ctx, s.production.Store, PlanID, id, resource, persistedDoorResource); err != nil {
		return nil, err
	}
	existing, err := s.production.Store.Load(ctx, id)
	if err == nil {
		if !terminalExecution(existing.Status) {
			if _, bundleErr := s.bundles.bundleForExecution(existing); bundleErr != nil {
				return nil, bundleErr
			}
		}
		return existing, nil
	}
	if !errors.Is(err, adgo.ErrExecutionNotFound) {
		return nil, err
	}
	return s.bundles.active.engine.StartOrLoad(ctx, id, map[string]any{"request": request}, adgo.BudgetLimit{})
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
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) ResolveUnlockApproval(
	ctx context.Context,
	executionID string,
	decision domainaction.ApprovalDecision,
	actor, reason string,
) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if bundle.bindings.unlockApprovalNode == "" {
		return nil, fmt.Errorf("%w: unlock approval execution=%s", ErrBundleOperationUnsupported, executionID)
	}
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
	if _, err := bundle.engine.ResolveHuman(ctx, executionID, bundle.bindings.unlockApprovalNode, adgo.HumanResolution{
		Decision: mapped, Actor: actor, Reason: reason,
	}); err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) ResolveReconciliation(
	ctx context.Context,
	executionID, nodeID string,
	decision domainaction.ReconcileDecision,
	actor, reason string,
) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if !bundle.bindings.supportsReconciliation(nodeID) {
		return nil, fmt.Errorf("%w: node %q is not reconcilable", ErrBundleOperationUnsupported, nodeID)
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
	if _, err := bundle.engine.ResolveHuman(ctx, executionID, nodeID, adgo.HumanResolution{
		Decision: mapped, Actor: actor, Reason: reason,
	}); err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}
