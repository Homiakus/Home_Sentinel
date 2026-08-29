package camera

import (
	"context"
	"errors"
	"fmt"

	"github.com/Homiakus/axiom/adgo"
)

var (
	ErrUnknownExecutionBundle     = errors.New("camera recovery: persisted execution bundle is not registered")
	ErrExecutionBundleMismatch    = errors.New("camera recovery: persisted execution identity does not match registered bundle")
	ErrBundleOperationUnsupported = errors.New("camera recovery: operation is not supported by execution bundle")
)

type executionBindings struct {
	operatorNode string
}

type bundleSpec struct {
	plan     *adgo.Plan
	registry *adgo.Registry
	bindings executionBindings
}

type executionBundle struct {
	plan     *adgo.Plan
	registry *adgo.Registry
	bindings executionBindings
	engine   *adgo.Engine
}

type bundleCatalog struct {
	active   *executionBundle
	byDigest map[string]*executionBundle
	ordered  []*executionBundle
}

func cameraV1BundleSpec(deps Dependencies) (bundleSpec, error) {
	plan, err := CompilePlan()
	if err != nil {
		return bundleSpec{}, fmt.Errorf("compile camera v1 plan: %w", err)
	}
	if plan.ID != PlanID || plan.Version != PlanVersion || plan.Digest != cameraV1PlanDigest {
		return bundleSpec{}, fmt.Errorf(
			"camera recovery: v1 plan identity drifted: got=%s/%s/%s want=%s/%s/%s",
			plan.ID, plan.Version, plan.Digest,
			PlanID, PlanVersion, cameraV1PlanDigest,
		)
	}
	return bundleSpec{
		plan:     plan,
		registry: newRegistryV1(deps),
		bindings: executionBindings{operatorNode: NodeOperator},
	}, nil
}

func newBundleCatalog(
	host *adgo.Host,
	engineOptions []adgo.EngineOption,
	activeDigest string,
	specs ...bundleSpec,
) (*bundleCatalog, error) {
	if host == nil {
		return nil, errors.New("camera recovery: bundle catalog requires host")
	}
	if len(specs) == 0 {
		return nil, errors.New("camera recovery: bundle catalog requires at least one bundle")
	}
	catalog := &bundleCatalog{byDigest: map[string]*executionBundle{}}
	for _, spec := range specs {
		if spec.plan == nil || spec.registry == nil {
			return nil, errors.New("camera recovery: execution bundle is incomplete")
		}
		if _, exists := catalog.byDigest[spec.plan.Digest]; exists {
			return nil, fmt.Errorf("camera recovery: duplicate bundle digest %s", spec.plan.Digest)
		}
		engine, err := host.Register(spec.plan, spec.registry, engineOptions...)
		if err != nil {
			return nil, fmt.Errorf("register camera bundle %s/%s: %w", spec.plan.ID, spec.plan.Version, err)
		}
		bundle := &executionBundle{
			plan:     spec.plan,
			registry: spec.registry,
			bindings: spec.bindings,
			engine:   engine,
		}
		catalog.byDigest[spec.plan.Digest] = bundle
		catalog.ordered = append(catalog.ordered, bundle)
		if spec.plan.Digest == activeDigest {
			catalog.active = bundle
		}
	}
	if catalog.active == nil {
		return nil, fmt.Errorf("camera recovery: active bundle digest %s is not registered", activeDigest)
	}
	return catalog, nil
}

func (c *bundleCatalog) bundleForExecution(execution *adgo.Execution) (*executionBundle, error) {
	if c == nil || execution == nil {
		return nil, ErrUnknownExecutionBundle
	}
	bundle := c.byDigest[execution.PlanDigest]
	if bundle == nil {
		return nil, fmt.Errorf("%w: execution=%s digest=%s", ErrUnknownExecutionBundle, execution.ID, execution.PlanDigest)
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
	if bundle.engine == nil {
		return nil, errors.New("camera recovery: execution bundle engine is not registered")
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

func (s *Service) executionBundle(ctx context.Context, executionID string) (*executionBundle, *adgo.Execution, error) {
	if s == nil || s.production == nil || s.bundles == nil {
		return nil, nil, errors.New("camera recovery: service is not open")
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

func (s *Service) validatePersistedExecutions(ctx context.Context) error {
	if s == nil || s.production == nil || s.bundles == nil {
		return errors.New("camera recovery: service is not open")
	}
	catalog, ok := s.production.Store.(adgo.ExecutionCatalog)
	if !ok {
		return errors.New("camera recovery: durable store does not expose an execution catalog")
	}
	ids, err := catalog.ListExecutionIDs(ctx)
	if err != nil {
		return fmt.Errorf("list persisted camera executions: %w", err)
	}
	for _, id := range ids {
		execution, err := s.production.Store.Load(ctx, id)
		if errors.Is(err, adgo.ErrExecutionNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("load persisted camera execution %s: %w", id, err)
		}
		if terminalExecution(execution.Status) {
			continue
		}
		if _, err := s.bundles.bundleForExecution(execution); err != nil {
			return fmt.Errorf("validate persisted camera execution %s: %w", id, err)
		}
	}
	return nil
}
