package door

import (
	"time"

	"github.com/Homiakus/axiom/adgo"
)

const (
	PlanID      = "home-sentinel-door-action"
	PlanVersion = "1"

	ActivityValidate = "DoorValidateRequest"
	DecisionRoute    = "DoorRouteRequest"
	ActivityApply    = "DoorApplyDesiredState"
	ActivityRecord   = "DoorRecordResult"

	NodeValidate      = "validate-request"
	NodeRoute         = "route-request"
	NodeApplyLock     = "apply-lock"
	NodeApproveUnlock = "approve-unlock"
	NodeApplyUnlock   = "apply-unlock"
	NodeRecordDone    = "record-done"
	NodeRecordDenied  = "record-denied"
	NodeRecordAborted = "record-aborted"

	UnlockApprovalEvent = "door.unlock.approval"
)

func CompilePlan() (*adgo.Plan, error) {
	retry := adgo.DefaultRetryPolicy()
	apply := func(id, next string) adgo.Node {
		return adgo.Node{
			ID: id, Kind: adgo.NodeActivity, Activity: ActivityApply,
			Requires: []string{"request"},
			Next:     []adgo.Transition{{To: next}},
			ExternalEffect: true,
			Risk:           adgo.RiskMedium,
			Timeout:        10 * time.Second,
			Retry:          retry,
			IdempotencyKey: "{execution}:{node}",
			ResourceKeys:   []string{"door-control"},
		}
	}
	record := func(id string) adgo.Node {
		return adgo.Node{ID: id, Kind: adgo.NodeActivity, Activity: ActivityRecord, Requires: []string{"request"}}
	}
	return adgo.Compile(adgo.Definition{
		ID:          PlanID,
		Version:     PlanVersion,
		InitialData: []string{"request"},
		Metadata: map[string]string{
			"purpose": "safe desired-state door control with reconciliation",
		},
		Nodes: []adgo.Node{
			{
				ID: NodeValidate, Kind: adgo.NodeActivity, Activity: ActivityValidate,
				Requires: []string{"request"}, Produces: []string{"request_valid"},
				Next: []adgo.Transition{{To: NodeRoute}},
			},
			{
				ID: NodeRoute, Kind: adgo.NodeDecision, Activity: DecisionRoute,
				Requires: []string{"request", "request_valid"},
				Next: []adgo.Transition{
					{To: NodeApplyLock, Outcome: adgo.OutcomePass},
					{To: NodeApproveUnlock, Outcome: adgo.OutcomeHuman},
				},
			},
			apply(NodeApplyLock, NodeRecordDone),
			{
				ID: NodeApproveUnlock, Kind: adgo.NodeHuman,
				Human: &adgo.HumanSpec{EventType: UnlockApprovalEvent, Risk: adgo.RiskHigh},
				Next: []adgo.Transition{
					{To: NodeApplyUnlock, Outcome: adgo.OutcomePass},
					{To: NodeRecordDenied, Outcome: adgo.OutcomeRejected},
					{To: NodeRecordAborted, Outcome: adgo.OutcomeCanceled},
				},
			},
			apply(NodeApplyUnlock, NodeRecordDone),
			record(NodeRecordDone),
			record(NodeRecordDenied),
			record(NodeRecordAborted),
		},
	})
}
