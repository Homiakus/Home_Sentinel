package camera

import (
	"time"

	"github.com/Homiakus/axiom/adgo"
)

const (
	PlanID      = "home-sentinel-camera-recovery"
	PlanVersion = "1"

	ActivityValidate  = "CameraRecoveryValidate"
	ActivityProbeNet  = "CameraRecoveryProbeNetwork"
	ActivityProbeRTSP = "CameraRecoveryProbeStream"
	ActivityReconnect = "CameraRecoveryReconnect"
	ActivityRecord    = "CameraRecoveryRecord"
	DecisionNetwork   = "CameraRecoveryDecideNetwork"
	DecisionStream    = "CameraRecoveryDecideStream"

	NodeValidate       = "validate-request"
	NodeProbeNet       = "probe-network"
	NodeDecideNet      = "decide-network"
	NodeProbeStream    = "probe-stream"
	NodeDecideStream   = "decide-stream"
	NodeReconnect      = "reconnect"
	NodeVerify         = "verify-stream"
	NodeDecideVerify   = "decide-verify"
	NodeOperator       = "operator-recovery"
	NodeReconnectFinal = "reconnect-after-operator"
	NodeVerifyFinal    = "verify-after-operator"
	NodeDecideFinal    = "decide-final"
	NodeHealthy        = "record-healthy"
	NodeRecovered      = "record-recovered"
	NodeFailed         = "record-failed"
	NodeRejected       = "record-rejected"
	NodeCanceled       = "record-canceled"

	OperatorEvent = "camera.recovery.operator"
)

func CompilePlan() (*adgo.Plan, error) {
	retry := adgo.DefaultRetryPolicy()
	probe := func(id, activity, output, next string) adgo.Node {
		return adgo.Node{
			ID: id, Kind: adgo.NodeActivity, Activity: activity,
			Requires: []string{"request"}, Produces: []string{output},
			Next: []adgo.Transition{{To: next}},
			Timeout: 5 * time.Second, Retry: retry,
		}
	}
	reconnect := func(id, next string) adgo.Node {
		return adgo.Node{
			ID: id, Kind: adgo.NodeActivity, Activity: ActivityReconnect,
			Requires: []string{"request"}, Next: []adgo.Transition{{To: next}},
			ExternalEffect: true, Risk: adgo.RiskMedium,
			Timeout: 10 * time.Second, Retry: retry,
			IdempotencyKey: "{execution}:{node}", ResourceKeys: []string{"camera-recovery"},
		}
	}
	record := func(id string) adgo.Node {
		return adgo.Node{ID: id, Kind: adgo.NodeActivity, Activity: ActivityRecord, Requires: []string{"request"}}
	}
	return adgo.Compile(adgo.Definition{
		ID: PlanID, Version: PlanVersion, InitialData: []string{"request"},
		Metadata: map[string]string{"purpose": "bounded stateful camera recovery"},
		Nodes: []adgo.Node{
			{ID: NodeValidate, Kind: adgo.NodeActivity, Activity: ActivityValidate, Requires: []string{"request"}, Produces: []string{"request_valid"}, Next: []adgo.Transition{{To: NodeProbeNet}}},
			probe(NodeProbeNet, ActivityProbeNet, "network_ok", NodeDecideNet),
			{ID: NodeDecideNet, Kind: adgo.NodeDecision, Activity: DecisionNetwork, Requires: []string{"network_ok"}, Next: []adgo.Transition{{To: NodeProbeStream, Outcome: adgo.OutcomePass}, {To: NodeOperator, Outcome: adgo.OutcomeHuman}}},
			probe(NodeProbeStream, ActivityProbeRTSP, "stream_ok", NodeDecideStream),
			{ID: NodeDecideStream, Kind: adgo.NodeDecision, Activity: DecisionStream, Requires: []string{"stream_ok"}, Next: []adgo.Transition{{To: NodeHealthy, Outcome: adgo.OutcomePass}, {To: NodeReconnect, Outcome: adgo.OutcomeRepair}}},
			reconnect(NodeReconnect, NodeVerify),
			probe(NodeVerify, ActivityProbeRTSP, "verify_ok", NodeDecideVerify),
			{ID: NodeDecideVerify, Kind: adgo.NodeDecision, Activity: DecisionStream, Requires: []string{"verify_ok"}, Next: []adgo.Transition{{To: NodeRecovered, Outcome: adgo.OutcomePass}, {To: NodeOperator, Outcome: adgo.OutcomeHuman}}},
			{ID: NodeOperator, Kind: adgo.NodeHuman, Human: &adgo.HumanSpec{EventType: OperatorEvent, Risk: adgo.RiskMedium}, Next: []adgo.Transition{{To: NodeReconnectFinal, Outcome: adgo.OutcomePass}, {To: NodeRejected, Outcome: adgo.OutcomeRejected}, {To: NodeCanceled, Outcome: adgo.OutcomeCanceled}}},
			reconnect(NodeReconnectFinal, NodeVerifyFinal),
			probe(NodeVerifyFinal, ActivityProbeRTSP, "verify_final_ok", NodeDecideFinal),
			{ID: NodeDecideFinal, Kind: adgo.NodeDecision, Activity: DecisionStream, Requires: []string{"verify_final_ok"}, Next: []adgo.Transition{{To: NodeRecovered, Outcome: adgo.OutcomePass}, {To: NodeFailed, Outcome: adgo.OutcomeHuman}}},
			record(NodeHealthy),
			record(NodeRecovered),
			record(NodeFailed),
			record(NodeRejected),
			record(NodeCanceled),
		},
	})
}
