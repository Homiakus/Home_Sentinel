// Package resourceguard prevents two independent durable executions from
// owning the same physical resource at the same time.
//
// It deliberately reserves a resource for the whole non-terminal execution,
// including human waits and ambiguous-side-effect reconciliation. This is
// stricter than an activity-scoped mutex and is the safe default for actuators.
package resourceguard

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Homiakus/axiom/adgo"
)

var (
	ErrBusy               = errors.New("resourceguard: resource is reserved by another execution")
	ErrCatalogUnsupported = errors.New("resourceguard: durable store does not expose execution catalog")
)

type BusyError struct {
	Resource    string
	ExecutionID string
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("%s: resource=%q execution=%q", ErrBusy, e.Resource, e.ExecutionID)
}

func (e *BusyError) Unwrap() error { return ErrBusy }

// Resolver extracts a canonical physical resource key from a persisted
// execution. It is only called for executions of planID.
type Resolver func(*adgo.Execution) (string, error)

// Check rejects candidateExecutionID when another non-terminal execution of the
// same plan owns resource. The check fails closed if persisted state cannot be
// decoded, because silently ignoring corrupt ownership data could permit two
// conflicting physical commands.
//
// The caller must serialize Check+StartOrLoad within one process. Multi-process
// control planes additionally require a distributed admission/fencing layer;
// this package intentionally does not pretend a process-local critical section
// is distributed locking.
func Check(
	ctx context.Context,
	store adgo.Store,
	planID string,
	candidateExecutionID string,
	resource string,
	resolve Resolver,
) error {
	if store == nil {
		return errors.New("resourceguard: store is required")
	}
	if strings.TrimSpace(planID) == "" || strings.TrimSpace(candidateExecutionID) == "" || strings.TrimSpace(resource) == "" {
		return errors.New("resourceguard: plan, execution and resource are required")
	}
	if resolve == nil {
		return errors.New("resourceguard: resolver is required")
	}
	catalog, ok := store.(adgo.ExecutionCatalog)
	if !ok {
		return ErrCatalogUnsupported
	}
	ids, err := catalog.ListExecutionIDs(ctx)
	if err != nil {
		return fmt.Errorf("resourceguard: list executions: %w", err)
	}
	for _, id := range ids {
		if id == candidateExecutionID {
			continue
		}
		execution, err := store.Load(ctx, id)
		if errors.Is(err, adgo.ErrExecutionNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("resourceguard: load execution %q: %w", id, err)
		}
		if execution.PlanID != planID || terminal(execution.Status) {
			continue
		}
		owned, err := resolve(execution)
		if err != nil {
			return fmt.Errorf("resourceguard: resolve resource for non-terminal execution %q: %w", id, err)
		}
		if owned == resource {
			return &BusyError{Resource: resource, ExecutionID: id}
		}
	}
	return nil
}

func terminal(status adgo.ExecutionStatus) bool {
	switch status {
	case adgo.StatusCompleted, adgo.StatusFailed, adgo.StatusCanceled, adgo.StatusDeadlocked:
		return true
	default:
		return false
	}
}
