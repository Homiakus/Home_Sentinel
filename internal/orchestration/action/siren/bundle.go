package siren

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/axiom/adgo"
)

var (
	ErrUnknownExecutionBundle  = errors.New("siren: persisted execution bundle is not registered")
	ErrExecutionBundleMismatch = errors.New("siren: persisted execution identity does not match reconstructed bundle")
)

const planVersionPrefix = "1-"

type executionBundle struct {
	plan   *adgo.Plan
	engine *adgo.Engine
}

type bundleCatalog struct {
	active        *executionBundle
	byDigest      map[string]*executionBundle
	deps          Dependencies
	host          *adgo.Host
	engineOptions []adgo.EngineOption
}

func newBundleCatalog(
	activePlan *adgo.Plan,
	deps Dependencies,
	host *adgo.Host,
	engineOptions []adgo.EngineOption,
) (*bundleCatalog, error) {
	if activePlan == nil || host == nil {
		return nil, errors.New("siren: execution bundle catalog requires active plan and host")
	}
	catalog := &bundleCatalog{
		byDigest:      map[string]*executionBundle{},
		deps:          deps,
		host:          host,
		engineOptions: append([]adgo.EngineOption(nil), engineOptions...),
	}
	bundle, err := catalog.registerPlan(activePlan)
	if err != nil {
		return nil, fmt.Errorf("register active siren execution bundle: %w", err)
	}
	catalog.active = bundle
	return catalog, nil
}

func parsePlanVersionDuration(version string) (time.Duration, error) {
	if !strings.HasPrefix(version, planVersionPrefix) {
		return 0, fmt.Errorf("siren: unsupported plan version %q", version)
	}
	encoded := strings.TrimPrefix(version, planVersionPrefix)
	if encoded == "" {
		return 0, fmt.Errorf("siren: plan version duration is empty")
	}
	duration, err := time.ParseDuration(encoded)
	if err != nil {
		return 0, fmt.Errorf("siren: parse plan version duration %q: %w", encoded, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("siren: plan version duration must be positive")
	}
	if canonical := planVersionPrefix + duration.String(); canonical != version {
		return 0, fmt.Errorf("siren: non-canonical plan version %q; want %q", version, canonical)
	}
	return duration, nil
}

func (c *bundleCatalog) registerPlan(plan *adgo.Plan) (*executionBundle, error) {
	if c == nil || c.host == nil || plan == nil {
		return nil, errors.New("siren: execution bundle is incomplete")
	}
	if existing := c.byDigest[plan.Digest]; existing != nil {
		return existing, nil
	}
	engine, err := c.host.Register(plan, NewRegistry(c.deps), c.engineOptions...)
	if err != nil {
		return nil, err
	}
	bundle := &executionBundle{plan: plan, engine: engine}
	c.byDigest[plan.Digest] = bundle
	return bundle, nil
}

func (c *bundleCatalog) ensureExecutionBundle(execution *adgo.Execution) (*executionBundle, error) {
	if c == nil || execution == nil {
		return nil, ErrUnknownExecutionBundle
	}
	if bundle := c.byDigest[execution.PlanDigest]; bundle != nil {
		return validateExecutionBundle(bundle, execution)
	}

	duration, err := parsePlanVersionDuration(execution.PlanVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: execution=%s: %v", ErrUnknownExecutionBundle, execution.ID, err)
	}
	plan, err := CompilePlan(duration)
	if err != nil {
		return nil, fmt.Errorf("%w: execution=%s: reconstruct plan: %v", ErrUnknownExecutionBundle, execution.ID, err)
	}
	if plan.ID != execution.PlanID || plan.Version != execution.PlanVersion || plan.Digest != execution.PlanDigest {
		return nil, fmt.Errorf(
			"%w: execution=%s persisted=%s/%s/%s reconstructed=%s/%s/%s",
			ErrExecutionBundleMismatch,
			execution.ID,
			execution.PlanID,
			execution.PlanVersion,
			execution.PlanDigest,
			plan.ID,
			plan.Version,
			plan.Digest,
		)
	}
	bundle, err := c.registerPlan(plan)
	if err != nil {
		return nil, fmt.Errorf("register reconstructed siren bundle %s: %w", plan.Version, err)
	}
	return validateExecutionBundle(bundle, execution)
}

func (c *bundleCatalog) bundleForExecution(execution *adgo.Execution) (*executionBundle, error) {
	if c == nil || execution == nil {
		return nil, ErrUnknownExecutionBundle
	}
	bundle := c.byDigest[execution.PlanDigest]
	if bundle == nil {
		return nil, fmt.Errorf("%w: execution=%s digest=%s", ErrUnknownExecutionBundle, execution.ID, execution.PlanDigest)
	}
	return validateExecutionBundle(bundle, execution)
}

func validateExecutionBundle(bundle *executionBundle, execution *adgo.Execution) (*executionBundle, error) {
	if bundle == nil || bundle.plan == nil || bundle.engine == nil || execution == nil {
		return nil, ErrUnknownExecutionBundle
	}
	if execution.PlanID != bundle.plan.ID || execution.PlanVersion != bundle.plan.Version || execution.PlanDigest != bundle.plan.Digest {
		return nil, fmt.Errorf(
			"%w: execution=%s persisted=%s/%s/%s registered=%s/%s/%s",
			ErrExecutionBundleMismatch,
			execution.ID,
			execution.PlanID,
			execution.PlanVersion,
			execution.PlanDigest,
			bundle.plan.ID,
			bundle.plan.Version,
			bundle.plan.Digest,
		)
	}
	return bundle, nil
}

func terminalExecution(status adgo.ExecutionStatus) bool {
	switch status {
	case adgo.StatusCompleted, adgo.StatusFailed, adgo.StatusCanceled, adgo.StatusDeadlocked:
		return true
	default:
		return false
	}
}

func (s *Service) loadPersistedBundles(ctx context.Context) error {
	if s == nil || s.production == nil || s.bundles == nil {
		return errors.New("siren: service is not open")
	}
	catalog, ok := s.production.Store.(adgo.ExecutionCatalog)
	if !ok {
		return errors.New("siren: durable store does not expose an execution catalog")
	}
	ids, err := catalog.ListExecutionIDs(ctx)
	if err != nil {
		return fmt.Errorf("list persisted siren executions: %w", err)
	}
	for _, id := range ids {
		execution, err := s.production.Store.Load(ctx, id)
		if errors.Is(err, adgo.ErrExecutionNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("load persisted siren execution %s: %w", id, err)
		}
		if terminalExecution(execution.Status) {
			continue
		}
		if _, err := s.bundles.ensureExecutionBundle(execution); err != nil {
			return fmt.Errorf("validate persisted siren execution %s: %w", id, err)
		}
	}
	return nil
}

func (s *Service) executionBundle(ctx context.Context, executionID string) (*executionBundle, *adgo.Execution, error) {
	if s == nil || s.production == nil || s.bundles == nil {
		return nil, nil, errors.New("siren: service is not open")
	}
	execution, err := s.production.Store.Load(ctx, executionID)
	if err != nil {
		return nil, nil, err
	}
	bundle, err := s.bundles.bundleForExecution(execution)
	if err != nil {
		return nil, execution, err
	}
	return bundle, execution, nil
}
