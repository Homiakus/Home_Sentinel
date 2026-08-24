package incident

import (
	"time"

	"github.com/Homiakus/axiom/adgo"
)

const (
	PlanID      = "home-sentinel-incident"
	PlanVersion = "1"

	ActivityNormalize = "IncidentNormalizeTrigger"
	ActivityCorrelate = "IncidentCorrelateEvidence"
	ActivityAssess    = "IncidentAssessRisk"
	ActivityNotify    = "IncidentNotifyOwner"
	ActivityArchive   = "IncidentArchive"

	NodeNormalize = "normalize-trigger"
	NodeCorrelate = "correlate-evidence"
	NodeAssess    = "assess-risk"
	NodeNotify    = "notify-owner"
	NodeAwait     = "await-owner-response"
	NodeArchive   = "archive-incident"

	OwnerResponseEvent = "incident.owner.response"
)

func CompilePlan() (*adgo.Plan, error) {
	retry := adgo.DefaultRetryPolicy()
	return adgo.Compile(adgo.Definition{
		ID:                PlanID,
		Version:           PlanVersion,
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
				ID: NodeNormalize, Kind: adgo.NodeActivity, Activity: ActivityNormalize,
				Requires: []string{"trigger"}, Produces: []string{"trigger_valid"},
				Next: []adgo.Transition{{To: NodeCorrelate}},
				ExpectedQualityGain: 0.1, CriticalPathWeight: 5,
			},
			{
				ID: NodeCorrelate, Kind: adgo.NodeActivity, Activity: ActivityCorrelate,
				Requires: []string{"trigger_valid"},
				Produces: []string{"evidence_count"},
				Next: []adgo.Transition{{To: NodeAssess}},
				ExpectedQualityGain: 0.2, CriticalPathWeight: 4,
			},
			{
				ID: NodeAssess, Kind: adgo.NodeActivity, Activity: ActivityAssess,
				Requires: []string{"evidence_count"},
				Produces: []string{"risk", "risk_score", "incident_summary"},
				Next: []adgo.Transition{{To: NodeNotify}},
				ExpectedQualityGain: 0.3, CriticalPathWeight: 3,
			},
			{
				ID: NodeNotify, Kind: adgo.NodeActivity, Activity: ActivityNotify,
				Requires: []string{"risk", "incident_summary"},
				Next: []adgo.Transition{{To: NodeAwait}},
				ExternalEffect: true,
				Risk:           adgo.RiskMedium,
				Timeout:        10 * time.Second,
				Retry:          retry,
				IdempotencyKey: "{execution}:{node}",
				ResourceKeys:   []string{"owner-notification"},
			},
			{
				ID: NodeAwait, Kind: adgo.NodeWait,
				Wait: &adgo.WaitSpec{EventType: OwnerResponseEvent},
				Next: []adgo.Transition{{To: NodeArchive}},
			},
			{
				ID: NodeArchive, Kind: adgo.NodeActivity, Activity: ActivityArchive,
				Requires: []string{"risk"}, Produces: []string{"archived"},
			},
		},
	})
}
