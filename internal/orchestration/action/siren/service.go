package siren

import (
	"context"
	"encoding/json"
	"errors"
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
	host       *adgo.Host
	bundles    *bundleCatalog
	worker     adgo.WorkerSpec
	startMu    sync.Mutex
}

func Open(config Config, deps Dependencies) (*Service, error) {
	maxDuration := config.MaxActivationDuration
	if maxDuration <= 0 {
		maxDuration = DefaultMaxActivation
	}
	activePlan, err := CompilePlan(maxDuration)
	if err != nil {
		return nil, fmt.Errorf("compile siren plan: %w", err)
	}
	production, err := adgo.OpenProduction(activePlan, NewRegistry(deps), config.Production)
	if err != nil {
		return nil, fmt.Errorf("open siren runtime: %w", err)
	}
	fail := func(err error) (*Service, error) {
		_ = production.Close()
		return nil, err
	}
	host, err := adgo.NewHost(production.Store)
	if err != nil {
		return fail(fmt.Errorf("open siren multi-plan host: %w", err))
	}
	engineOptions := []adgo.EngineOption{
		adgo.WithEngineLeaseTTL(config.Production.LeaseTTL),
		adgo.WithEnginePollInterval(config.Production.PollInterval),
		adgo.WithCoordinatorInterval(config.Production.CoordinatorInterval),
		adgo.WithMaxLeaseRecoveries(config.Production.MaxLeaseRecoveries),
		adgo.WithAdaptiveRouter(production.Router),
	}
	bundles, err := newBundleCatalog(activePlan, deps, host, engineOptions)
	if err != nil {
		return fail(err)
	}

	workerID := strings.TrimSpace(config.WorkerID)
	if workerID == "" {
		workerID = "home-sentinel-siren"
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
	if err := service.loadPersistedBundles(context.Background()); err != nil {
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

func (s *Service) Start(ctx context.Context, request domainaction.SirenRequest) (*adgo.Execution, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if s == nil || s.production == nil || s.bundles == nil || s.bundles.active == nil || s.bundles.active.engine == nil {
		return nil, errors.New("siren: service is not open")
	}
	id := domainaction.SirenExecutionID(request)
	resource := sirenResourceKey(request.SirenID)

	s.startMu.Lock()
	defer s.startMu.Unlock()
	if err := resourceguard.Check(ctx, s.production.Store, PlanID, id, resource, persistedSirenResource); err != nil {
		return nil, err
	}
	current, err := s.production.Store.Load(ctx, id)
	if err == nil {
		if !terminalExecution(current.Status) {
			if _, bundleErr := s.bundles.bundleForExecution(current); bundleErr != nil {
				return nil, bundleErr
			}
		}
		return current, nil
	}
	if !errors.Is(err, adgo.ErrExecutionNotFound) {
		return nil, err
	}
	return s.bundles.active.engine.StartOrLoad(ctx, id, map[string]any{"request": request}, adgo.BudgetLimit{})
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
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return bundle.engine.RunLocal(ctx, executionID, adgo.LocalRunOptions{Worker: s.worker})
}

// Stop requests cancellation through the exact engine pinned by the persisted
// execution digest. ADGO then runs that plan's enable-node compensation, which
// is an idempotent ensure-disabled operation.
func (s *Service) Stop(ctx context.Context, executionID, reason string) (*adgo.Execution, error) {
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if _, err := bundle.engine.Cancel(ctx, executionID, reason); err != nil {
		return nil, err
	}
	return bundle.engine.Get(ctx, executionID)
}

func (s *Service) Get(ctx context.Context, executionID string) (*adgo.Execution, error) {
	if s == nil || s.production == nil || s.bundles == nil {
		return nil, errors.New("siren: service is not open")
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
	return s.serveRuntime(ctx)
}

func (s *Service) servePreflight(ctx context.Context) error {
	if s == nil || s.host == nil || s.production == nil || s.production.ScheduleRunner == nil {
		return errors.New("siren: service is not open")
	}
	return s.loadPersistedBundles(ctx)
}

func (s *Service) serveRuntime(ctx context.Context) error {
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- s.host.ServeResilient(serveCtx, s.worker) }()
	go func() { errCh <- s.production.ScheduleRunner.Run(serveCtx) }()
	err := <-errCh
	cancel()
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
