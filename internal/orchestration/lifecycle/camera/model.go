package camera

import (
	"github.com/Homiakus/axiom/model"
)

const (
	StateConnecting = "connecting"
	StateOnline     = "online"
	StateDegraded   = "degraded"
	StateOffline    = "offline"
	StateDisabled   = "disabled"
)

type State struct {
	Status    string `json:"status"`
	Enabled   bool   `json:"enabled"`
	LastError string `json:"lastError"`
}

type Connected struct {
	Endpoint string `json:"endpoint"`
}

type StreamDegraded struct {
	Reason string `json:"reason"`
}

type StreamFailed struct {
	Reason string `json:"reason"`
}

type Recovered struct {
	Probe string `json:"probe"`
}

type DisableRequested struct {
	Reason string `json:"reason"`
}

type EnableRequested struct {
	Reason string `json:"reason"`
}

var (
	statusKey    = model.Key[State, string]("Status")
	enabledKey   = model.Key[State, bool]("Enabled")
	lastErrorKey = model.Key[State, string]("LastError")

	degradedReasonKey = model.Key[StreamDegraded, string]("Reason")
	failedReasonKey   = model.Key[StreamFailed, string]("Reason")
)

// Definition builds the Axiom model for one camera lifecycle. Domain/media
// packages never import this package; it is an orchestration adapter only.
func Definition() *model.Definition {
	definition := model.New("HomeSentinelCamera").Version("1")
	camera := model.Bind[State](definition, "Camera")

	connected := model.EventOf[Connected](definition)
	degraded := model.EventOf[StreamDegraded](definition)
	failed := model.EventOf[StreamFailed](definition)
	recovered := model.EventOf[Recovered](definition)
	disable := model.EventOf[DisableRequested](definition)
	enable := model.EventOf[EnableRequested](definition)

	model.StateDefault(camera, statusKey, StateConnecting)
	model.StateDefault(camera, enabledKey, true)
	model.StateDefault(camera, lastErrorKey, "")

	status := model.StateField(camera, statusKey)
	enabledState := model.StateField(camera, enabledKey)
	lastError := model.StateField(camera, lastErrorKey)
	degradedReason := model.EventField(degraded, degradedReasonKey)
	failedReason := model.EventField(failed, failedReasonKey)

	definition.Rule("connected").
		On(connected.Trigger()).
		When(enabledState.EQ(true)).
		Set(status, StateOnline).
		Set(lastError, "")

	definition.Rule("degraded").
		On(degraded.Trigger()).
		When(enabledState.EQ(true)).
		Set(status, StateDegraded).
		Set(lastError, degradedReason)

	definition.Rule("failed").
		On(failed.Trigger()).
		When(enabledState.EQ(true)).
		Set(status, StateOffline).
		Set(lastError, failedReason)

	definition.Rule("recovered").
		On(recovered.Trigger()).
		When(enabledState.EQ(true)).
		Set(status, StateOnline).
		Set(lastError, "")

	definition.Rule("disable").
		On(disable.Trigger()).
		Set(enabledState, false).
		Set(status, StateDisabled)

	definition.Rule("enable").
		On(enable.Trigger()).
		When(enabledState.EQ(false)).
		Set(enabledState, true).
		Set(status, StateConnecting).
		Set(lastError, "")

	definition.Claim(
		"disabledCameraHasDisabledStatus",
		model.Implies(enabledState.EQ(false), status.EQ(StateDisabled)),
	)

	return definition
}
