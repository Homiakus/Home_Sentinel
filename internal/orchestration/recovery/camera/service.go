package camera

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
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
	host       *adgo.Host
	bundles    *bundleCatalog
	worker     adgo.WorkerSpec
	startMu    sync.Mutex
}

func Open(config Config, deps Dependencies) (*Service, error) {
	active, err := cameraV1BundleSpec(deps)
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
		return nil, errors.New("camera recovery: active execution bundle is incomplete")
	}
	production, err := adgo.OpenProduction(active.plan, active.registry, config.Production)
	if err != nil {
		return nil, fmt.Errorf("open camera recovery runtime: %w", err)
	}
	fail := func(err error) (*Service, error) {
		_ = production.Close()
		return nil, err
	}
	host, err := adgo.NewHost(production.Store)
	if err != nil {
		return fail(fmt.Errorf("open camera recovery multi-plan host: %w", err))
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
		workerID = "home-sentinel-camera-recovery"
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

func (s *Service) Start(ctx context.Context, request domainrecovery.CameraRequest) (*adgo.Execution, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if s == nil || s.production == nil || s.bundles == nil || s.bundles.active == nil || s.bundles.active.engine == nil {
		return nil, errors.New("camera recovery: service is not open")
	}
	id := domainrecovery.CameraExecutionID(request)
	resource := cameraResourceKey(request.CameraID)

	s.startMu.Lock()
	defer s.startMu.Unlock()
	if err := resourceguard.Check(ctx, s.production.Store, PlanID, id, resource, persistedCameraResource); err != nil {
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

func cameraResourceKey(cameraID string) string {
	return "camera-recovery:" + strings.TrimSpace(cameraID)
}

func persistedCameraResource(execution *adgo.Execution) (string, error) {
	raw, ok := execution.Data["request"]
	if !ok {
		return "", fmt.Errorf("camera recovery: persisted request is missing")
	}
	var request domainrecovery.CameraRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return "", fmt.Errorf("camera recovery: decode persisted request: %w", err)
	}
	if strings.TrimSpace(request.CameraID) == "" {
		return "", fmt.Errorf("camera recovery: persisted camera id is empty")
	}
	return cameraResourceKey(request.CameraID), nil
}

func (s *Service) Drive(ctx context.Context, executionID string) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) ResolveOperator(
	ctx context.Context,
	executionID string,
	decision domainrecovery.OperatorDecision,
	actor, reason string,
) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if bundle.bindings.operatorNode == "" {
		return nil, fmt.Errorf("%w: operator resolution execution=%s", ErrBundleOperationUnsupported, executionID)
	}
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
	if _, err := bundle.engine.ResolveHuman(ctx, executionID, bundle.bindings.operatorNode, adgo.HumanResolution{
		Decision: mapped, Actor: actor, Reason: reason,
	}); err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

func (s *Service) Get(ctx context.Context, executionID string) (*adgo.Execution, error) {
	if s == nil || s.production == nil || s.bundles == nil {
		return nil, errors.New("camera recovery: service is not open")
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

func (s *Service) Serve(ctx context.Context) error {
	if err := s.servePreflight(ctx); err != nil {
		return err
	}
	return s.host.ServeResilient(ctx, s.worker)
}

func (s *Service) servePreflight(ctx context.Context) error {
	if s == nil || s.production == nil || s.host == nil || s.bundles == nil {
		return errors.New("camera recovery: service is not open")
	}
	return s.validatePersistedExecutions(ctx)
}
