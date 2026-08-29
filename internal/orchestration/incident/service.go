package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	host       *adgo.Host
	bundles    *bundleCatalog
	worker     adgo.WorkerSpec
}

func Open(config Config, deps Dependencies) (*Service, error) {
	bundles, err := newBundleCatalog(deps)
	if err != nil {
		return nil, err
	}
	production, err := adgo.OpenProduction(bundles.active.plan, bundles.active.registry, config.Production)
	if err != nil {
		return nil, fmt.Errorf("open incident production runtime: %w", err)
	}
	fail := func(err error) (*Service, error) {
		_ = production.Close()
		return nil, err
	}
	host, err := adgo.NewHost(production.Store)
	if err != nil {
		return fail(fmt.Errorf("open incident multi-plan host: %w", err))
	}
	engineOptions := []adgo.EngineOption{
		adgo.WithEngineLeaseTTL(config.Production.LeaseTTL),
		adgo.WithEnginePollInterval(config.Production.PollInterval),
		adgo.WithCoordinatorInterval(config.Production.CoordinatorInterval),
		adgo.WithMaxLeaseRecoveries(config.Production.MaxLeaseRecoveries),
		adgo.WithAdaptiveRouter(production.Router),
	}
	for _, bundle := range bundles.ordered {
		engine, registerErr := host.Register(bundle.plan, bundle.registry, engineOptions...)
		if registerErr != nil {
			return fail(fmt.Errorf("register incident execution bundle %s: %w", bundle.plan.Version, registerErr))
		}
		bundle.engine = engine
	}

	concurrency := config.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	workerID := strings.TrimSpace(config.WorkerID)
	if workerID == "" {
		workerID = "home-sentinel-local"
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

func (s *Service) Start(ctx context.Context, trigger domainincident.Trigger) (*adgo.Execution, error) {
	if err := trigger.Validate(); err != nil {
		return nil, err
	}
	if s == nil || s.production == nil || s.bundles == nil || s.bundles.active == nil || s.bundles.active.engine == nil {
		return nil, errors.New("incident: service is not open")
	}
	id := domainincident.ExecutionID(trigger)
	current, err := s.production.Store.Load(ctx, id)
	if err == nil {
		if _, bundleErr := s.bundles.bundleForExecution(current); bundleErr != nil {
			return nil, bundleErr
		}
		return current, nil
	}
	if !errors.Is(err, adgo.ErrExecutionNotFound) {
		return nil, err
	}
	return s.bundles.active.engine.StartOrLoad(ctx, id, map[string]any{"trigger": trigger}, adgo.BudgetLimit{})
}

// Drive executes currently runnable work using the exact engine pinned by the
// execution's persisted PlanDigest.
func (s *Service) Drive(ctx context.Context, executionID string) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) OwnerResponseBindingNode(ctx context.Context, executionID string) (string, error) {
	bundle, _, err := s.executionBundle(ctx, strings.TrimSpace(executionID))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bundle.bindings.ownerResponseNode) == "" {
		return "", fmt.Errorf("%w: owner response for plan version %s", ErrBundleOperationUnsupported, bundle.plan.Version)
	}
	return bundle.bindings.ownerResponseNode, nil
}

func (s *Service) OwnerDecisionBindingNode(ctx context.Context, executionID string) (string, error) {
	bundle, _, err := s.executionBundle(ctx, strings.TrimSpace(executionID))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bundle.bindings.ownerDecisionNode) == "" {
		return "", fmt.Errorf("%w: owner decision for plan version %s", ErrBundleOperationUnsupported, bundle.plan.Version)
	}
	return bundle.bindings.ownerDecisionNode, nil
}

func (s *Service) OwnerResponse(ctx context.Context, executionID, eventID string, payload any) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	target := bundle.bindings.ownerResponseNode
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("%w: owner response for plan version %s", ErrBundleOperationUnsupported, bundle.plan.Version)
	}
	var raw json.RawMessage
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode owner response: %w", err)
		}
		raw = encoded
	}
	if err := bundle.engine.Signal(ctx, executionID, adgo.Event{
		ID: eventID, Type: OwnerResponseEvent, TargetNode: target, Payload: raw,
	}); err != nil {
		return nil, err
	}
	return s.Drive(ctx, executionID)
}

func (s *Service) ResolveOwnerDecision(
	ctx context.Context,
	executionID string,
	decision domainincident.Decision,
	actor string,
	reason string,
	payload any,
) (*adgo.Execution, error) {
	mapped, err := mapDecision(decision)
	if err != nil {
		return nil, err
	}
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	target := bundle.bindings.ownerDecisionNode
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("%w: owner decision for plan version %s", ErrBundleOperationUnsupported, bundle.plan.Version)
	}
	if _, err := bundle.engine.ResolveHuman(ctx, executionID, target, adgo.HumanResolution{
		Decision: mapped,
		Actor:    actor,
		Reason:   reason,
		Payload:  payload,
	}); err != nil {
		return nil, err
	}
	return s.Drive(ctx, executionID)
}

func mapDecision(value domainincident.Decision) (adgo.HumanDecision, error) {
	switch value {
	case domainincident.DecisionApprove:
		return adgo.HumanApprove, nil
	case domainincident.DecisionEdit:
		return adgo.HumanEdit, nil
	case domainincident.DecisionReject:
		return adgo.HumanReject, nil
	case domainincident.DecisionRetry:
		return adgo.HumanRetry, nil
	case domainincident.DecisionConfirm, domainincident.DecisionAcknowledge:
		return adgo.HumanConfirm, nil
	case domainincident.DecisionAbort:
		return adgo.HumanAbort, nil
	default:
		return "", fmt.Errorf("incident: unsupported owner decision %q", value)
	}
}

func (s *Service) Get(ctx context.Context, executionID string) (*adgo.Execution, error) {
	if s == nil || s.production == nil {
		return nil, errors.New("incident: service is not open")
	}
	execution, err := s.production.Store.Load(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if !terminalExecution(execution.Status) {
		if _, err := s.bundles.bundleForExecution(execution); err != nil {
			return nil, err
		}
	}
	return execution, nil
}

// Serve runs a multi-plan resilient coordinator/worker plus the active-v2
// schedule runner. Fail-closed validation is completed synchronously before any
// long-running goroutine starts so startup failures cannot be hidden by a live
// coordinator loop.
func (s *Service) Serve(ctx context.Context) error {
	if err := s.servePreflight(ctx); err != nil {
		return err
	}
	return s.serveRuntime(ctx)
}

func (s *Service) servePreflight(ctx context.Context) error {
	if s == nil || s.host == nil || s.production == nil || s.production.ScheduleRunner == nil {
		return errors.New("incident: service is not open")
	}
	return s.validatePersistedExecutions(ctx)
}

func (s *Service) serveRuntime(ctx context.Context) error {
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- s.host.ServeResilient(serveCtx, s.worker) }()
	go func() { errCh <- s.production.ScheduleRunner.Run(serveCtx) }()
	err := <-errCh
	cancel()
	return normalizeServeError(ctx, err)
}

func normalizeServeError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
