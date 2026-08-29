package incident

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/axiom/adgo"
)

const (
	planVersionV1 = "1"

	nodeNormalizeV1 = "normalize-trigger"
	nodeCorrelateV1 = "correlate-evidence"
	nodeAssessV1    = "assess-risk"
	nodeNotifyV1    = "notify-owner"
	nodeAwaitV1     = "await-owner-response"
	nodeArchiveV1   = "archive-incident"

	legacyV1PlanDigest = "sha256:1688786c8de8ef9af76a0baf39c987c3434032998eb612a708a96be92f8e615d"
)

// compilePlanV1 is the immutable incident definition shipped by caa446b2.
// Its digest is a durable compatibility boundary for executions created before
// the v2 risk-routing plan was introduced.
func compilePlanV1() (*adgo.Plan, error) {
	retry := adgo.DefaultRetryPolicy()
	return adgo.Compile(adgo.Definition{
		ID:                PlanID,
		Version:           planVersionV1,
		InitialData:       []string{"trigger"},
		GlobalConcurrency: 4,
		ActivityLimits: map[string]int{
			ActivityNotify: 2,
		},
		Metadata: map[string]string{
			"owner":   "Home_Sentinel",
			"purpose": "durable security incident processing",
		},
		Nodes: []adgo.Node{
			{
				ID: nodeNormalizeV1, Kind: adgo.NodeActivity, Activity: ActivityNormalize,
				Requires: []string{"trigger"}, Produces: []string{"trigger_valid"},
				Next:                []adgo.Transition{{To: nodeCorrelateV1}},
				ExpectedQualityGain: 0.1, CriticalPathWeight: 5,
			},
			{
				ID: nodeCorrelateV1, Kind: adgo.NodeActivity, Activity: ActivityCorrelate,
				Requires: []string{"trigger_valid"}, Produces: []string{"evidence_count"},
				Next:                []adgo.Transition{{To: nodeAssessV1}},
				ExpectedQualityGain: 0.2, CriticalPathWeight: 4,
			},
			{
				ID: nodeAssessV1, Kind: adgo.NodeActivity, Activity: ActivityAssess,
				Requires:            []string{"evidence_count"},
				Produces:            []string{"risk", "risk_score", "incident_summary"},
				Next:                []adgo.Transition{{To: nodeNotifyV1}},
				ExpectedQualityGain: 0.3, CriticalPathWeight: 3,
			},
			{
				ID: nodeNotifyV1, Kind: adgo.NodeActivity, Activity: ActivityNotify,
				Requires:       []string{"risk", "incident_summary"},
				Next:           []adgo.Transition{{To: nodeAwaitV1}},
				ExternalEffect: true,
				Risk:           adgo.RiskMedium,
				Timeout:        10 * time.Second,
				Retry:          retry,
				IdempotencyKey: "{execution}:{node}",
				ResourceKeys:   []string{"owner-notification"},
			},
			{
				ID: nodeAwaitV1, Kind: adgo.NodeWait,
				Wait: &adgo.WaitSpec{EventType: OwnerResponseEvent},
				Next: []adgo.Transition{{To: nodeArchiveV1}},
			},
			{
				ID: nodeArchiveV1, Kind: adgo.NodeActivity, Activity: ActivityArchive,
				Requires: []string{"risk"}, Produces: []string{"archived"},
			},
		},
	})
}

func newRegistryV1(deps Dependencies) *adgo.Registry {
	registry := adgo.NewRegistry()
	registry.Activity(ActivityNormalize, normalizeTrigger)
	registry.Activity(ActivityCorrelate, correlateEvidence)
	registry.Activity(ActivityAssess, assessRiskV1)
	registry.Activity(ActivityNotify, notifyOwner(deps.Notifier))
	registry.Activity(ActivityArchive, archiveIncidentV1)
	return registry
}

// assessRiskV1 preserves the exact pre-risk-policy scoring semantics used by
// durable v1 executions. The branch-free clamps are equivalent to the original
// positive-evidence and score-cap guards while avoiding unobservable boundary
// mutants at contribution=0 and score=1.
func assessRiskV1(_ context.Context, req adgo.ActivityRequest) (adgo.ActivityResult, error) {
	trigger, err := readData[domainincident.Trigger](req.Data, "trigger")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}
	evidenceCount, err := readData[int](req.Data, "evidence_count")
	if err != nil {
		return adgo.ActivityResult{}, adgo.Fail(adgo.FailureInvalidInput, err)
	}

	score := trigger.Confidence * 0.55
	if strings.Contains(strings.ToLower(trigger.Kind), "person") {
		score += 0.30
	}
	evidenceContribution := math.Min(0.15, math.Max(0, float64(evidenceCount)*0.05))
	score = math.Min(1, score+evidenceContribution)

	risk := classifyRiskV1(score)
	summary := fmt.Sprintf(
		"%s from %s; confidence=%.3f evidence=%d risk=%s",
		trigger.Kind, trigger.SourceID, trigger.Confidence, evidenceCount, risk,
	)
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

func classifyRiskV1(score float64) domainincident.Risk {
	thresholds := [...]float64{0.50, 0.75, 0.90}
	risks := [...]domainincident.Risk{
		domainincident.RiskLow,
		domainincident.RiskMedium,
		domainincident.RiskHigh,
		domainincident.RiskCritical,
	}
	// Search for the first threshold >= the next representable value. Advancing
	// by one ULP preserves the historical inclusive >= threshold semantics while
	// keeping the compatibility classifier free of mutation-invisible case guards.
	index := sort.SearchFloat64s(thresholds[:], math.Nextafter(score, math.Inf(1)))
	return risks[index]
}

func archiveIncidentV1(_ context.Context, _ adgo.ActivityRequest) (adgo.ActivityResult, error) {
	return adgo.ActivityResult{
		Facts:   map[string]any{"archived": true},
		Outcome: adgo.OutcomeCompleted,
	}, nil
}
