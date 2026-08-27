package siren

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	"github.com/Homiakus/axiom/adgo"
)

type Dependencies struct {
	Siren gateway.SirenController
}

func NewRegistry(deps Dependencies) *adgo.Registry {
	registry := adgo.NewRegistry()
	registry.Activity(ActivityValidate, validateRequest)
	registry.Activity(ActivityEnable, setEnabled(deps.Siren, true))
	registry.Activity(ActivityDisable, setEnabled(deps.Siren, false))
	registry.Activity(ActivityRecord, recordResult)
	registry.Compensation(CompEnsureOff, ensureDisabled(deps.Siren))
	return registry
}

func validateRequest(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	request, err := readData[domainaction.SirenRequest](req.Data, "request")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	if err := request.Validate(); err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	return adgo.ActivityResult{Facts: map[string]any{"request_valid": true}, Outcome: adgo.OutcomeCompleted}, nil
}

func setEnabled(controller gateway.SirenController, desired bool) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if controller == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("siren: controller is not configured"))
		}
		request, err := readData[domainaction.SirenRequest](req.Data, "request")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}
		before, err := controller.Enabled(ctx, request.SirenID)
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureTransient, fmt.Errorf("read siren state: %w", err))
		}
		if before == desired {
			return adgo.ActivityResult{Facts: map[string]any{"siren_enabled": desired}, Outcome: adgo.OutcomeCompleted}, nil
		}
		result, err := controller.SetEnabled(ctx, gateway.Operation{
			ExecutionID: req.ExecutionID, IdempotencyKey: req.IdempotencyKey,
		}, request.SirenID, desired)
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureAmbiguousSideEffect, fmt.Errorf("siren write outcome unknown: %w", err))
		}
		after, verifyErr := controller.Enabled(ctx, request.SirenID)
		if verifyErr != nil || after != desired {
			return adgo.ActivityResult{}, adgo.Fail(
				adgo.FailureAmbiguousSideEffect,
				fmt.Errorf("siren state verification failed: provider=%s observed=%v verifyErr=%v", result.ProviderID, after, verifyErr),
			)
		}
		return adgo.ActivityResult{
			Facts:   map[string]any{"siren_enabled": desired, "siren_provider_id": result.ProviderID},
			Outcome: adgo.OutcomeCompleted,
		}, nil
	}
}

func ensureDisabled(controller gateway.SirenController) adgo.CompensationHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) error {
		if controller == nil {
			return errors.New("siren compensation: controller is not configured")
		}
		request, err := readData[domainaction.SirenRequest](req.Data, "request")
		if err != nil {
			return err
		}
		enabled, err := controller.Enabled(ctx, request.SirenID)
		if err != nil {
			return fmt.Errorf("siren compensation read: %w", err)
		}
		if !enabled {
			return nil
		}
		if _, err := controller.SetEnabled(ctx, gateway.Operation{
			ExecutionID: req.ExecutionID, IdempotencyKey: req.IdempotencyKey,
		}, request.SirenID, false); err != nil {
			return fmt.Errorf("siren compensation disable: %w", err)
		}
		enabled, err = controller.Enabled(ctx, request.SirenID)
		if err != nil {
			return fmt.Errorf("siren compensation verify: %w", err)
		}
		if enabled {
			return errors.New("siren compensation: siren is still enabled")
		}
		return nil
	}
}

func recordResult(_ context.Context, _ adgo.ActivityRequest) (adgo.ActivityResult, error) {
	return adgo.ActivityResult{Outcome: adgo.OutcomeCompleted}, nil
}

func readData[T any](data map[string]json.RawMessage, key string) (T, error) {
	var zero T
	raw, ok := data[key]
	if !ok {
		return zero, fmt.Errorf("siren: required fact %q is missing", key)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, fmt.Errorf("siren: decode fact %q: %w", key, err)
	}
	return value, nil
}
