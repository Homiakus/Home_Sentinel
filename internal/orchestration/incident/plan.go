package incident

import (
	"time"

	"github.com/Homiakus/axiom/adgo"
)

const (
	PlanID      = "home-sentinel-incident"
	PlanVersion = "2"

	ActivityNormalize = "IncidentNormalizeTrigger"
	ActivityCorrelate = "IncidentCorrelateEvidence"
	ActivityAssess    = "IncidentAssessRisk"
	DecisionRouteRisk = "IncidentRouteRisk"
	ActivityNotify    = "IncidentNotifyOwner"
	ActivityArchive   = "IncidentArchive"

	NodeNormalize       = "normalize-trigger"
	NodeCorrelate       = "correlate-evidence"
	NodeAssess          = "assess-risk"
	NodeRouteRisk       = "route-risk"
	NodeArchiveLow      = "archive-low-risk"
	NodeNotifyMedium    = "notify-medium-risk"
	NodeAwaitAck        = "await-owner-ack"
	NodeArchiveMedium   = "archive-medium-risk"
	NodeNotifyHigh      = "notify-high-risk"
	NodeHumanDecision   = "owner-high-risk-decision"
	NodeArchiveHigh     = "archive-high-risk"
	NodeArchiveRejected = "archive-rejected"
	NodeArchiveCanceled = "archive-canceled"

	OwnerResponseEvent = "incident.owner.response"
	OwnerDecisionEvent = "incident.owner.decision"
)

func CompilePlan() (*adgo.Plan, error) {
	retry := adgo.DefaultRetryPolicy()
	notify := func(id, next string) adgo.Node {
		return adgo.Node{
			ID: id, Kind: adgo.NodeActivity, Activity: ActivityNotify,
			Requires: []string{"risk", "incident_summary"},
			Next:     []adgo.Transition{{To: next}},
			ExternalEffect: true,
			Risk:           adgo.RiskMedium,
			Timeout:        10 * time.Second,
			Retry:          retry,
			IdempotencyKey: "{execution}:{node}",
			ResourceKeys:   []string{"owner-notification"},
		}
	}
	archive := func(id string) adgo.Node {
		return adgo.Node{ID: id, Kind: adgo.NodeActivity, Activity: ActivityArchive, Requires: []string{"risk"}}
	}

	return adgo.Compile(adgo.Definition{
		ID:                PlanID,
		Version:           PlanVersion,
		InitialData:       []string{"trigger"},
		GlobalConcurrency: 4,
		ActivityLimits: map[string]int{
			ActivityNotify: 2,
		},
		Metadata: map[string]string{
			"owner":       "Home_Sentinel",
			"purpose":     "durable security incident processing",
			"risk_policy": "risk-v2",
		},
		Nodes: []adgo.Node{
			{
				ID: NodeNormalize, Kind: adgo.NodeActivity, Activity: ActivityNormalize,
				Requires: []string{"trigger"}, Produces: []string{"trigger_valid"},
				Next: []adgo.Transition{{To: NodeCorrelate}},
				ExpectedQualityGain: 0.1, CriticalPathWeight: 5,
			},
			{
				ID: NodeCorrelate, Kind: adgo.NodeActivity, Activity: ActivityCorrelate,
				Requires: []string{"trigger_valid"}, Produces: []string{"evidence_count"},
				Next: []adgo.Transition{{To: NodeAssess}},
				ExpectedQualityGain: 0.2, CriticalPathWeight: 4,
			},
			{
				ID: NodeAssess, Kind: adgo.NodeActivity, Activity: ActivityAssess,
				Requires: []string{"evidence_count"},
				Produces: []string{"risk", "risk_score", "risk_assessment", "incident_summary"},
				Next: []adgo.Transition{{To: NodeRouteRisk}},
				ExpectedQualityGain: 0.3, CriticalPathWeight: 3,
			},
			{
				ID: NodeRouteRisk, Kind: adgo.NodeDecision, Activity: DecisionRouteRisk,
				Requires: []string{"risk"},
				Next: []adgo.Transition{
					{To: NodeArchiveLow, Outcome: adgo.OutcomeCompleted},
					{To: NodeNotifyMedium, Outcome: adgo.OutcomePass},
					{To: NodeNotifyHigh, Outcome: adgo.OutcomeHuman},
				},
			},
			archive(NodeArchiveLow),
			notify(NodeNotifyMedium, NodeAwaitAck),
			{
				ID: NodeAwaitAck, Kind: adgo.NodeWait,
				Wait: &adgo.WaitSpec{EventType: OwnerResponseEvent},
				Next: []adgo.Transition{{To: NodeArchiveMedium}},
			},
			archive(NodeArchiveMedium),
			notify(NodeNotifyHigh, NodeHumanDecision),
			{
				ID: NodeHumanDecision, Kind: adgo.NodeHuman,
				Human: &adgo.HumanSpec{EventType: OwnerDecisionEvent, Risk: adgo.RiskHigh},
				Next: []adgo.Transition{
					{To: NodeArchiveHigh, Outcome: adgo.OutcomePass},
					{To: NodeArchiveRejected, Outcome: adgo.OutcomeRejected},
					{To: NodeArchiveCanceled, Outcome: adgo.OutcomeCanceled},
				},
			},
			archive(NodeArchiveHigh),
			archive(NodeArchiveRejected),
			archive(NodeArchiveCanceled),
		},
	})
}
