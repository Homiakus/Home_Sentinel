package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	riskpolicy "github.com/Homiakus/Home_Sentinel/internal/policy/risk"
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
	registry.Decision(DecisionRouteRisk, routeRisk)
	registry.Activity(ActivityNotify, notifyOwner(deps.Notifier))
	registry.Activity(ActivityArchive, archiveIncident)
	return registry
}

func normalizeTrigger(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	trigger, err := readData[domainincident.Trigger](req.Data, "trigger")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	if err := trigger.Validate(); err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	return adgo.ActivityResult{Facts: map[string]any{"trigger_valid": true}, Outcome: adgo.OutcomeCompleted}, nil
}

func correlateEvidence(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	trigger, err := readData[domainincident.Trigger](req.Data, "trigger")
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
	trigger, err := readData[domainincident.Trigger](req.Data, "trigger")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	evidenceCount, err := readData[int](req.Data, "evidence_count")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	assessment, err := riskpolicy.DefaultPolicy().Assess(riskpolicy.FeaturesFromTrigger(trigger, evidenceCount))
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	summary := fmt.Sprintf(
		"%s from %s; confidence=%.3f evidence=%d risk=%s score=%.3f policy=%s",
		trigger.Kind, trigger.SourceID, trigger.Confidence, evidenceCount,
		assessment.Risk, assessment.Score, assessment.PolicyVersion,
	)
	return adgo.ActivityResult{
		Facts: map[string]any{
			"risk":             assessment.Risk,
			"risk_score":       assessment.Score,
			"risk_assessment":  assessment,
			"incident_summary": summary,
		},
		Quality: adgo.QualityVector{"risk_input_quality": trigger.Confidence},
		Outcome: adgo.OutcomeCompleted,
	}, nil
}

func routeRisk(_ context.Context, snapshot adgo.Snapshot) (adgo.Outcome, error) {
	risk, err := readData[domainincident.Risk](snapshot.Data, "risk")
	if err != nil {
		return adgo.OutcomeFail, err
	}
	switch risk {
	case domainincident.RiskLow:
		return adgo.OutcomeCompleted, nil
	case domainincident.RiskMedium:
		return adgo.OutcomePass, nil
	case domainincident.RiskHigh, domainincident.RiskCritical:
		return adgo.OutcomeHuman, nil
	default:
		return adgo.OutcomeFail, fmt.Errorf("incident: unsupported risk %q", risk)
	}
}

func notifyOwner(notifier gateway.Notifier) adgo.ActivityHandler {
	return func(ctx context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
		if notifier == nil {
			return adgo.ActivityResult{}, adgo.Fail(adgo.FailurePermanent, errors.New("incident: notifier is not configured"))
		}
		risk, err := readData[domainincident.Risk](req.Data, "risk")
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
			Facts:   map[string]any{"notification_provider_id": result.ProviderID},
			Outcome: adgo.OutcomeCompleted,
		}, nil
	}
}

func archiveIncident(_ context.Context, _ adgo.ActivityRequest) (adgo.ActivityResult, error) {
	return adgo.ActivityResult{Outcome: adgo.OutcomeCompleted}, nil
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
