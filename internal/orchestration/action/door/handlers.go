package door

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
	Door gateway.DoorController
}

func NewRegistry(deps Dependencies) *adgo.Registry {
	registry := adgo.NewRegistry()
	registry.Activity(ActivityValidate, validateRequest)
	registry.Decision(DecisionRoute, routeRequest)
	registry.Activity(ActivityApply, applyDesiredState(deps.Door))
	registry.Activity(ActivityRecord, recordResult)
	return registry
}

func validateRequest(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	request, err := readData[domainaction.DoorRequest](req.Data, "request")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	if err := request.Validate(); err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	return adgo.ActivityResult{Facts: map[string]any{"request_valid": true}, Outcome: adgo.OutcomeCompleted}, nil
}

func routeRequest(_ context.Context, snapshot adgo.Snapshot) (adgo.Outcome, error) {
	request, err := readData[domainaction.DoorRequest](snapshot.Data, "request")
	if err != nil {
		return adgo.OutcomeFail, err
	}
	switch request.Desired {
	case gateway.LockLocked:
		return adgo.OutcomePass, nil
	case gateway.LockUnlocked:
		return adgo.OutcomeHuman, nil
	default:
		return adgo.OutcomeFail, fmt.Errorf("door action: unsupported desired state %q", request.Desired)
	}
}

func applyDesiredState(controller gateway.DoorController) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if controller == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("door action: controller is not configured"))
		}
		request, err := readData[domainaction.DoorRequest](req.Data, "request")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}

		before, err := controller.LockState(ctx, request.DoorID)
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureTransient, fmt.Errorf("read lock state before write: %w", err))
		}
		if before == request.Desired {
			return adgo.ActivityResult{
				Facts: map[string]any{"door_effect": gateway.EffectAlreadyApplied, "door_state": request.Desired},
				Outcome: adgo.OutcomeCompleted,
			}, nil
		}

		result, err := controller.SetLockState(ctx, gateway.Operation{
			ExecutionID: req.ExecutionID, IdempotencyKey: req.IdempotencyKey,
		}, request.DoorID, request.Desired)
		if err != nil {
			// Once a write was attempted, a transport error cannot prove that the
			// physical device did not accept the command. Do not blind-retry.
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureAmbiguousSideEffect, fmt.Errorf("door write outcome unknown: %w", err))
		}

		after, verifyErr := controller.LockState(ctx, request.DoorID)
		if verifyErr == nil && after == request.Desired {
			return adgo.ActivityResult{
				Facts: map[string]any{
					"door_effect":      result.State,
					"door_state":       after,
					"door_provider_id": result.ProviderID,
				},
				Outcome: adgo.OutcomeCompleted,
			}, nil
		}

		return adgo.ActivityResult{}, adgo.Fail(
			adgo.FailureAmbiguousSideEffect,
			fmt.Errorf("door state could not be verified after write: provider=%s observed=%s verifyErr=%v", result.ProviderID, after, verifyErr),
		)
	}
}

func recordResult(_ context.Context, _ adgo.ActivityRequest) (adgo.ActivityResult, error) {
	return adgo.ActivityResult{Outcome: adgo.OutcomeCompleted}, nil
}

func readData[T any](data map[string]json.RawMessage, key string) (T, error) {
	var zero T
	raw, ok := data[key]
	if !ok {
		return zero, fmt.Errorf("door action: required fact %q is missing", key)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, fmt.Errorf("door action: decode fact %q: %w", key, err)
	}
	return value, nil
}
