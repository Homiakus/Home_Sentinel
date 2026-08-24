package siren

import (
	"fmt"
	"time"

	"github.com/Homiakus/axiom/adgo"
)

const (
	PlanID = "home-sentinel-siren-action"

	ActivityValidate = "SirenValidateRequest"
	ActivityEnable   = "SirenEnable"
	ActivityDisable  = "SirenDisable"
	ActivityRecord   = "SirenRecordResult"
	CompEnsureOff    = "SirenEnsureDisabled"

	NodeValidate = "validate-request"
	NodeEnable   = "enable-siren"
	NodeSafety   = "safety-timer"
	NodeDisable  = "disable-siren"
	NodeRecord   = "record-result"
)

func CompilePlan(maxActivation time.Duration) (*adgo.Plan, error) {
	if maxActivation <= 0 {
		return nil, fmt.Errorf("siren: max activation duration must be > 0")
	}
	retry := adgo.DefaultRetryPolicy()
	return adgo.Compile(adgo.Definition{
		ID:          PlanID,
		Version:     "1-" + maxActivation.String(),
		InitialData: []string{"request"},
		Metadata: map[string]string{
			"purpose":                 "bounded siren activation with fail-safe compensation",
			"max_activation_duration": maxActivation.String(),
		},
		Nodes: []adgo.Node{
			{
				ID: NodeValidate, Kind: adgo.NodeActivity, Activity: ActivityValidate,
				Requires: []string{"request"}, Produces: []string{"request_valid"},
				Next: []adgo.Transition{{To: NodeEnable}},
			},
			{
				ID: NodeEnable, Kind: adgo.NodeActivity, Activity: ActivityEnable,
				Requires: []string{"request", "request_valid"},
				Next: []adgo.Transition{{To: NodeSafety}},
				ExternalEffect: true,
				Risk:           adgo.RiskMedium,
				Timeout:        10 * time.Second,
				Retry:          retry,
				IdempotencyKey: "{execution}:{node}",
				ResourceKeys:   []string{"siren-control"},
				Compensation:   CompEnsureOff,
			},
			{
				ID: NodeSafety, Kind: adgo.NodeWait,
				Wait: &adgo.WaitSpec{Duration: maxActivation},
				Next: []adgo.Transition{{To: NodeDisable}},
			},
			{
				ID: NodeDisable, Kind: adgo.NodeActivity, Activity: ActivityDisable,
				Requires: []string{"request"},
				Next: []adgo.Transition{{To: NodeRecord}},
				ExternalEffect: true,
				Risk:           adgo.RiskMedium,
				Timeout:        10 * time.Second,
				Retry:          retry,
				IdempotencyKey: "{execution}:{node}",
				ResourceKeys:   []string{"siren-control"},
			},
			{ID: NodeRecord, Kind: adgo.NodeActivity, Activity: ActivityRecord, Requires: []string{"request"}},
		},
	})
}
