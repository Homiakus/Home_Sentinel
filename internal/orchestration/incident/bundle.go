package incident

import (
	"context"
	"errors"
	"fmt"

	"github.com/Homiakus/axiom/adgo"
)

var (
	ErrUnknownExecutionBundle     = errors.New("incident: persisted execution bundle is not registered")
	ErrExecutionBundleMismatch    = errors.New("incident: persisted execution identity does not match registered bundle")
	ErrBundleOperationUnsupported = errors.New("incident: operation is not supported by execution bundle")
)

type executionBindings struct {
	ownerResponseNode string
	ownerDecisionNode string
}

type executionBundle struct {
	plan     *adgo.Plan
	registry *adgo.Registry
	bindings executionBindings
	engine   *adgo.Engine
}

type bundleCatalog struct {
	active    *executionBundle
	byDigest  map[string]*executionBundle
	byVersion map[string]*executionBundle
	ordered   []*executionBundle
}

func newBundleCatalog(deps Dependencies) (*bundleCatalog, error) {
	legacyPlan, err := compilePlanV1()
	if err != nil {
		return nil, fmt.Errorf("compile incident v1 plan: %w", err)
	}
	if legacyPlan.Digest != legacyV1PlanDigest {
		return nil, fmt.Errorf(
			"incident: v1 plan digest drifted: got %s want %s",
			legacyPlan.Digest,
			legacyV1PlanDigest,
		)
	}
	activePlan, err := CompilePlan()
	if err != nil {
		return nil, fmt.Errorf("compile active incident plan: %w", err)
	}
	if activePlan.Digest == legacyPlan.Digest {
		return nil, errors.New("incident: v1 and active plan digests must be distinct")
	}

	legacy := &executionBundle{
		plan:     legacyPlan,
		registry: newRegistryV1(deps),
		bindings: executionBindings{ownerResponseNode: nodeAwaitV1},
	}
	active := &executionBundle{
		plan:     activePlan,
		registry: NewRegistry(deps),
		bindings: executionBindings{
			ownerResponseNode: NodeAwaitAck,
			ownerDecisionNode: NodeHumanDecision,
		},
	}
	catalog := &bundleCatalog{
		active:    active,
		byDigest:  map[string]*executionBundle{},
		byVersion: map[string]*executionBundle{},
		ordered:   []*executionBundle{legacy, active},
	}
	for _, bundle := range catalog.ordered {
		if bundle.plan == nil || bundle.registry == nil {
			return nil, errors.New("incident: execution bundle is incomplete")
		}
		if _, exists := catalog.byDigest[bundle.plan.Digest]; exists {
			return nil, fmt.Errorf("incident: duplicate bundle digest %s", bundle.plan.Digest)
		}
		if _, exists := catalog.byVersion[bundle.plan.Version]; exists {
			return nil, fmt.Errorf("incident: duplicate bundle version %s", bundle.plan.Version)
		}
		catalog.byDigest[bundle.plan.Digest] = bundle
		catalog.byVersion[bundle.plan.Version] = bundle
	}
	return catalog, nil
}

func (c *bundleCatalog) bundleForExecution(execution *adgo.Execution) (*executionBundle, error) {
	if c == nil || execution == nil {
		return nil, ErrUnknownExecutionBundle
	}
	bundle := c.byDigest[execution.PlanDigest]
	if bundle == nil {
		return nil, fmt.Errorf("%w: digest=%s execution=%s", ErrUnknownExecutionBundle, execution.PlanDigest, execution.ID)
	}
	if execution.PlanID != bundle.plan.ID || execution.PlanVersion != bundle.plan.Version {
		return nil, fmt.Errorf(
			"%w: execution=%s persisted=%s/%s registered=%s/%s digest=%s",
			ErrExecutionBundleMismatch,
			execution.ID,
			execution.PlanID,
			execution.PlanVersion,
			bundle.plan.ID,
			bundle.plan.Version,
			execution.PlanDigest,
		)
	}
	if bundle.engine == nil {
		return nil, errors.New("incident: execution bundle engine is not registered")
	}
	return bundle, nil
}

func (c *bundleCatalog) bundleForVersion(version string) (*executionBundle, error) {
	if c == nil {
		return nil, ErrUnknownExecutionBundle
	}
	bundle := c.byVersion[version]
	if bundle == nil {
		return nil, fmt.Errorf("%w: version=%s", ErrUnknownExecutionBundle, version)
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
		return nil, nil, errors.New("incident: service is not open")
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
		return errors.New("incident: service is not open")
	}
	catalog, ok := s.production.Store.(adgo.ExecutionCatalog)
	if !ok {
		return errors.New("incident: durable store does not expose an execution catalog")
	}
	ids, err := catalog.ListExecutionIDs(ctx)
	if err != nil {
		return fmt.Errorf("list persisted incident executions: %w", err)
	}
	for _, id := range ids {
		execution, err := s.production.Store.Load(ctx, id)
		if err != nil {
			if errors.Is(err, adgo.ErrExecutionNotFound) {
				continue
			}
			return fmt.Errorf("load persisted incident execution %s: %w", id, err)
		}
		if terminalExecution(execution.Status) {
			continue
		}
		if _, err := s.bundles.bundleForExecution(execution); err != nil {
			return fmt.Errorf("validate persisted incident execution %s: %w", id, err)
		}
	}
	return nil
}
