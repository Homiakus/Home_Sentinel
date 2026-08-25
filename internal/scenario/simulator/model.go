package simulator

import (
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type SimulationMode string

const (
	ModePure       SimulationMode = "pure"
	ModeHistorical SimulationMode = "historical_replay"
	ModeWhatIf     SimulationMode = "what_if"
	ModeShadow     SimulationMode = "shadow"
)

type HypotheticalEffect struct {
	CapabilityID string          `json:"capabilityId"`
	ResourceKey  string          `json:"resourceKey,omitempty"`
	Risk         model.RiskLevel `json:"risk"`
	Arguments    map[string]any  `json:"arguments,omitempty"`
	Action       string          `json:"action"` // "WOULD_EXECUTE"
	Timestamp    time.Time       `json:"timestamp"`
}

type TraceStep struct {
	StepID      string          `json:"stepId"`
	Kind        string          `json:"kind"`
	SystemOwned bool            `json:"systemOwned"`
	Outcome     string          `json:"outcome"` // "MATCH", "CONDITION_TRUE", "CONDITION_FALSE", "WOULD_EXECUTE", "WAITED", "APPROVED", "REJECTED", "TIMEOUT", "STOPPED"
	Inputs      map[string]any  `json:"inputs,omitempty"`
	Explanation string          `json:"explanation"`
	Timestamp   time.Time       `json:"timestamp"`
}

type SimulationContext struct {
	Mode            SimulationMode         `json:"mode"`
	TriggerEvent    map[string]model.Value `json:"triggerEvent,omitempty"`
	HomeState       map[string]model.Value `json:"homeState,omitempty"`
	HumanApprovals  map[string]bool        `json:"humanApprovals,omitempty"` // stepID -> approve/reject
	SimulatedTime   time.Time              `json:"simulatedTime"`
	Timezone        string                 `json:"timezone,omitempty"`
}

type SimulationResult struct {
	ScenarioID          string               `json:"scenarioId"`
	RevisionID          string               `json:"revisionId"`
	Mode                SimulationMode       `json:"mode"`
	Passed              bool                 `json:"passed"`
	StartTime           time.Time            `json:"startTime"`
	EndTime             time.Time            `json:"endTime"`
	SimulatedDuration   time.Duration        `json:"simulatedDuration"`
	Traces              []TraceStep          `json:"traces"`
	HypotheticalEffects []HypotheticalEffect `json:"hypotheticalEffects"`
	FinalOutcome        string               `json:"finalOutcome"`
	Errors              []string             `json:"errors,omitempty"`
}
