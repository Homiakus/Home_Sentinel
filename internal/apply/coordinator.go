package apply

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Plan struct {
	Summary string `json:"summary"`
	Changed bool   `json:"changed"`
	Details any    `json:"details,omitempty"`
}
type Backup struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type Target interface {
	Plan(ctx context.Context, desired []byte) (Plan, error)
	Validate(ctx context.Context, desired []byte, plan Plan) error
	Backup(ctx context.Context, plan Plan) (Backup, error)
	Apply(ctx context.Context, desired []byte, plan Plan) error
	Verify(ctx context.Context, desired []byte, plan Plan) error
	Commit(ctx context.Context, desired []byte, plan Plan) error
	Rollback(ctx context.Context, backup Backup, cause error) error
}

type Result struct {
	Plan       Plan
	Backup     Backup
	Applied    bool
	RolledBack bool
}
type Coordinator struct{}

func (Coordinator) Run(ctx context.Context, t Target, desired []byte) (Result, error) {
	if t == nil {
		return Result{}, errors.New("nil apply target")
	}
	plan, err := t.Plan(ctx, desired)
	if err != nil {
		return Result{}, fmt.Errorf("plan: %w", err)
	}
	result := Result{Plan: plan}
	if !plan.Changed {
		return result, nil
	}
	if err := t.Validate(ctx, desired, plan); err != nil {
		return result, fmt.Errorf("validate: %w", err)
	}
	backup, err := t.Backup(ctx, plan)
	if err != nil {
		return result, fmt.Errorf("backup: %w", err)
	}
	result.Backup = backup
	if err := t.Apply(ctx, desired, plan); err != nil {
		return rollback(ctx, t, result, backup, fmt.Errorf("apply: %w", err))
	}
	result.Applied = true
	if err := t.Verify(ctx, desired, plan); err != nil {
		return rollback(ctx, t, result, backup, fmt.Errorf("verify: %w", err))
	}
	if err := t.Commit(ctx, desired, plan); err != nil {
		return rollback(ctx, t, result, backup, fmt.Errorf("commit: %w", err))
	}
	return result, nil
}
func rollback(ctx context.Context, t Target, r Result, b Backup, cause error) (Result, error) {
	if err := t.Rollback(ctx, b, cause); err != nil {
		return r, errors.Join(cause, fmt.Errorf("rollback: %w", err))
	}
	r.RolledBack = true
	return r, cause
}
