package update

import (
	"context"
	"errors"
	"fmt"
)

type Checkpoint interface {
	Create(context.Context) (string, error)
	Restore(context.Context, string) error
}
type Executor interface {
	Stage(context.Context, Plan) error
	Activate(context.Context, Plan) error
	Verify(context.Context, Plan) error
	Rollback(context.Context, Plan) error
}
type Result struct {
	Checkpoint string `json:"checkpoint"`
	RolledBack bool   `json:"rolled_back"`
	Verified   bool   `json:"verified"`
}
type Manager struct {
	Checkpoint Checkpoint
	Executor   Executor
}

func (m Manager) Apply(ctx context.Context, p Plan) (Result, error) {
	if !p.Compatible {
		return Result{}, errors.New("update plan is incompatible")
	}
	if m.Checkpoint == nil || m.Executor == nil {
		return Result{}, errors.New("checkpoint and executor required")
	}
	cp, e := m.Checkpoint.Create(ctx)
	if e != nil {
		return Result{}, fmt.Errorf("create rollback checkpoint: %w", e)
	}
	if cp == "" {
		return Result{}, errors.New("checkpoint did not return rollback data")
	}
	res := Result{Checkpoint: cp}
	if e = m.Executor.Stage(ctx, p); e != nil {
		return res, fmt.Errorf("stage update: %w", e)
	}
	if e = m.Executor.Activate(ctx, p); e != nil {
		return res, m.rollback(ctx, p, cp, "activate", e, &res)
	}
	if e = m.Executor.Verify(ctx, p); e != nil {
		return res, m.rollback(ctx, p, cp, "verify", e, &res)
	}
	res.Verified = true
	return res, nil
}
func (m Manager) rollback(ctx context.Context, p Plan, cp, phase string, cause error, res *Result) error {
	rb := m.Executor.Rollback(ctx, p)
	rr := m.Checkpoint.Restore(ctx, cp)
	res.RolledBack = rb == nil && rr == nil
	if rb != nil || rr != nil {
		return fmt.Errorf("%s update failed: %v; rollback executor=%v restore=%v", phase, cause, rb, rr)
	}
	return fmt.Errorf("%s update failed and rollback completed: %w", phase, cause)
}
