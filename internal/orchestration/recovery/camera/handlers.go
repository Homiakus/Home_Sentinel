package camera

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	"github.com/Homiakus/axiom/adgo"
)

type Dependencies struct {
	Controller gateway.CameraRecoveryController
}

// NewRegistry is the active registry surface. Camera v1 handler semantics are
// deliberately frozen below. A real semantic v2 must introduce a separate
// versioned registry/handler set rather than changing these implementations in
// place, because PlanDigest covers the graph but not Go handler behavior.
func NewRegistry(deps Dependencies) *adgo.Registry {
	return newRegistryV1(deps)
}

func newRegistryV1(deps Dependencies) *adgo.Registry {
	registry := adgo.NewRegistry()
	registry.Activity(ActivityValidate, validateRequestV1)
	registry.Activity(ActivityProbeNet, probeNetworkV1(deps.Controller))
	registry.Activity(ActivityProbeRTSP, probeStreamV1(deps.Controller))
	registry.Activity(ActivityReconnect, reconnectV1(deps.Controller))
	registry.Activity(ActivityRecord, recordV1)
	registry.Decision(DecisionNetwork, decideBoolV1("network_ok", adgo.OutcomePass, adgo.OutcomeHuman))
	registry.Decision(DecisionStream, decideStreamV1)
	return registry
}

func validateRequestV1(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	request, err := readData[domainrecovery.CameraRequest](req.Data, "request")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	if err := request.Validate(); err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	return adgo.ActivityResult{Facts: map[string]any{"request_valid": true}, Outcome: adgo.OutcomeCompleted}, nil
}

func probeNetworkV1(controller gateway.CameraRecoveryController) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if controller == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("camera recovery: controller is not configured"))
		}
		request, err := readData[domainrecovery.CameraRequest](req.Data, "request")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}
		ok, err := controller.ProbeNetwork(ctx, request.CameraID)
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureTransient, err)
		}
		return adgo.ActivityResult{Facts: map[string]any{"network_ok": ok}, Outcome: adgo.OutcomeCompleted}, nil
	}
}

func probeStreamV1(controller gateway.CameraRecoveryController) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if controller == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("camera recovery: controller is not configured"))
		}
		request, err := readData[domainrecovery.CameraRequest](req.Data, "request")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}
		ok, err := controller.ProbeStream(ctx, request.CameraID)
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureTransient, err)
		}
		output := ""
		switch req.NodeID {
		case NodeProbeStream:
			output = "stream_ok"
		case NodeVerify:
			output = "verify_ok"
		case NodeVerifyFinal:
			output = "verify_final_ok"
		default:
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, fmt.Errorf("camera recovery: probe activity used by unexpected node %q", req.NodeID))
		}
		return adgo.ActivityResult{Facts: map[string]any{output: ok}, Outcome: adgo.OutcomeCompleted}, nil
	}
}

func reconnectV1(controller gateway.CameraRecoveryController) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if controller == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("camera recovery: controller is not configured"))
		}
		request, err := readData[domainrecovery.CameraRequest](req.Data, "request")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}
		result, err := controller.Reconnect(ctx, gateway.Operation{ExecutionID: req.ExecutionID, IdempotencyKey: req.IdempotencyKey}, request.CameraID)
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureAmbiguousSideEffect, fmt.Errorf("camera reconnect outcome unknown: %w", err))
		}
		if result.State == gateway.EffectAmbiguous {
			streamOK, probeErr := controller.ProbeStream(ctx, request.CameraID)
			if probeErr != nil || !streamOK {
				return adgo.ActivityResult{}, adgo.Fail(adgo.FailureAmbiguousSideEffect, fmt.Errorf("camera reconnect cannot be verified: %v", probeErr))
			}
		}
		return adgo.ActivityResult{Facts: map[string]any{"reconnect_provider_id": result.ProviderID}, Outcome: adgo.OutcomeCompleted}, nil
	}
}

func decideBoolV1(key string, yes, no adgo.Outcome) adgo.DecisionHandler {
	return func(_ context.Context, snapshot adgo.Snapshot) (adgo.Outcome, error) {
		value, err := readData[bool](snapshot.Data, key)
		if err != nil {
			return adgo.OutcomeFail, err
		}
		if value {
			return yes, nil
		}
		return no, nil
	}
}

func decideStreamV1(_ context.Context, snapshot adgo.Snapshot) (adgo.Outcome, error) {
	for _, item := range []struct {
		key string
		no  adgo.Outcome
	}{
		{key: "verify_final_ok", no: adgo.OutcomeHuman},
		{key: "verify_ok", no: adgo.OutcomeHuman},
		{key: "stream_ok", no: adgo.OutcomeRepair},
	} {
		if raw, ok := snapshot.Data[item.key]; ok {
			var value bool
			if err := json.Unmarshal(raw, &value); err != nil {
				return adgo.OutcomeFail, err
			}
			if value {
				return adgo.OutcomePass, nil
			}
			return item.no, nil
		}
	}
	return adgo.OutcomeFail, errors.New("camera recovery: no stream probe result")
}

func recordV1(_ context.Context, _ adgo.ActivityRequest) (adgo.ActivityResult, error) {
	return adgo.ActivityResult{Outcome: adgo.OutcomeCompleted}, nil
}

func readData[T any](data map[string]json.RawMessage, key string) (T, error) {
	var zero T
	raw, ok := data[key]
	if !ok {
		return zero, fmt.Errorf("camera recovery: required fact %q missing", key)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, fmt.Errorf("camera recovery: decode fact %q: %w", key, err)
	}
	return value, nil
}
