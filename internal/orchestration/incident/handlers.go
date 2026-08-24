package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	"github.com/Homiakus/axiom/adgo"
)

type Dependencies struct {
	Notifier gateway.Notifier
}

func NewRegistry(deps Dependencies) *adgo.Registry {
	registry := adgo.NewRegistry()
	registry.Activity(ActivityNormalize, normalizeTrigger)
	registry.Activity(ActivityCorrelate, correlateEvidence)
	registry.Activity(ActivityAssess, assessRisk)
	registry.Activity(ActivityNotify, notifyOwner(deps.Notifier))
	registry.Activity(ActivityArchive, archiveIncident)
	return registry
}

func normalizeTrigger(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	trigger, err := readData[incident.Trigger](req.Data, "trigger")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	if err := trigger.Validate(); err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	return adgo.ActivityResult{
		Facts:   map[string]any{"trigger_valid": true},
		Outcome: adgo.OutcomeCompleted,
	}, nil
}

func correlateEvidence(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	trigger, err := readData[incident.Trigger](req.Data, "trigger")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	artifacts := make(map[string]adgo.ArtifactRef, len(trigger.Artifacts))
	for i, ref := range trigger.Artifacts {
		artifacts[fmt.Sprintf("trigger-%d", i)] = adgo.ArtifactRef{
			URI: ref.URI, Digest: ref.Digest, Size: ref.Size, MediaType: ref.MediaType,
		}
	}
	return adgo.ActivityResult{
		Facts:     map[string]any{"evidence_count": len(artifacts)},
		Artifacts: artifacts,
		Outcome:   adgo.OutcomeCompleted,
	}, nil
}

func assessRisk(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	trigger, err := readData[incident.Trigger](req.Data, "trigger")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	evidenceCount, err := readData[int](req.Data, "evidence_count")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}

	score := trigger.Confidence * 0.55
	kind := strings.ToLower(trigger.Kind)
	if strings.Contains(kind, "person") {
		score += 0.30
	}
	if evidenceCount > 0 {
		score += math.Min(0.15, float64(evidenceCount)*0.05)
	}
	if score > 1 {
		score = 1
	}

	risk := incident.RiskLow
	switch {
	case score >= 0.90:
		risk = incident.RiskCritical
	case score >= 0.75:
		risk = incident.RiskHigh
	case score >= 0.50:
		risk = incident.RiskMedium
	}
	summary := fmt.Sprintf("%s from %s; confidence=%.3f evidence=%d risk=%s", trigger.Kind, trigger.SourceID, trigger.Confidence, evidenceCount, risk)
	return adgo.ActivityResult{
		Facts: map[string]any{
			"risk":             risk,
			"risk_score":       score,
			"incident_summary": summary,
		},
		Quality: adgo.QualityVector{"risk_input_quality": trigger.Confidence},
		Outcome: adgo.OutcomeCompleted,
	}, nil
}

func notifyOwner(notifier gateway.Notifier) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if notifier == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("incident: notifier is not configured"))
		}
		risk, err := readData[incident.Risk](req.Data, "risk")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}
		summary, err := readData[string](req.Data, "incident_summary")
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
		}
		operation := gateway.Operation{ExecutionID: req.ExecutionID, IdempotencyKey: req.IdempotencyKey}
		result, err := notifier.Notify(ctx, operation, gateway.Notification{
			Channel: "owner", Title: "Home Sentinel incident: " + string(risk), Body: summary,
		})
		if err != nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureTransient, err)
		}
		if result.State == gateway.EffectAmbiguous {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailureAmbiguousSideEffect, fmt.Errorf("notification effect ambiguous: %s", result.ProviderID))
		}
		return adgo.ActivityResult{
			Facts: map[string]any{"notification_provider_id": result.ProviderID},
			Outcome: adgo.OutcomeCompleted,
		}, nil
	}
}

func archiveIncident(_ context.Context, _ adgo.ActivityRequest) (adgo.ActivityResult, error) {
	return adgo.ActivityResult{Facts: map[string]any{"archived": true}, Outcome: adgo.OutcomeCompleted}, nil
}

func readData[T any](data map[string]json.RawMessage, key string) (T, error) {
	var zero T
	raw, ok := data[key]
	if !ok {
		return zero, fmt.Errorf("incident: required fact %q is missing", key)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, fmt.Errorf("incident: decode fact %q: %w", key, err)
	}
	return value, nil
}
