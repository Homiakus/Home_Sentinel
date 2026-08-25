package compiler

import (
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type Runtime string

const (
	RuntimeAxiom Runtime = "Axiom"
	RuntimeADGO  Runtime = "ADGO"
)

type ExternalEffectSpec struct {
	CapabilityID   string                     `json:"capabilityId"`
	ResourceKey    string                     `json:"resourceKey"`
	Permission     string                     `json:"permission"`
	Risk           model.RiskLevel            `json:"risk"`
	Idempotency    string                     `json:"idempotency"`
	Timeout        time.Duration              `json:"timeout"`
	MaxRetries     int                        `json:"maxRetries"`
	DesiredState   map[string]any             `json:"desiredState,omitempty"`
	ReadBeforeWrite bool                      `json:"readBeforeWrite"`
	VerifyAfterWrite bool                     `json:"verifyAfterWrite"`
	Reconciliation bool                       `json:"reconciliation"`
	Compensation   string                     `json:"compensation"`
}

type SafetyAugmentation struct {
	NodeID      string          `json:"nodeId"`
	SystemOwned bool            `json:"systemOwned"`
	Kind        string          `json:"kind"` // e.g. "human_approval", "resource_reservation", "duration_limit", "verify_write", "compensation"
	Description string          `json:"description"`
	Risk        model.RiskLevel `json:"risk"`
	Reason      string          `json:"reason"`
}

type TemporalRequirement struct {
	Kind        string        `json:"kind"` // e.g. "schedule", "debounce", "throttle", "wait", "timeout"
	Spec        string        `json:"spec"`
	Duration    time.Duration `json:"duration,omitempty"`
	Timezone    string        `json:"timezone,omitempty"`
	Durable     bool          `json:"durable"`
}

type MigrationInfo struct {
	CompatibleFromVersion string   `json:"compatibleFromVersion"`
	BreakingChanges       []string `json:"breakingChanges,omitempty"`
}

type GraphNode struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	SystemOwned bool            `json:"systemOwned"`
	Risk        model.RiskLevel `json:"risk"`
	Next        []string        `json:"next,omitempty"`
	Details     map[string]any  `json:"details,omitempty"`
}

type GraphRepresentation struct {
	EntryNode string      `json:"entryNode"`
	Nodes     []GraphNode `json:"nodes"`
}

type AxiomPlan struct {
	ModuleID   string         `json:"moduleId"`
	TriggerID  string         `json:"triggerId"`
	Condition  *model.Expr    `json:"condition,omitempty"`
	ActionCap  string         `json:"actionCapability"`
	ActionArgs map[string]any `json:"actionArgs,omitempty"`
}

type ADGONode struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"` // "Activity", "Wait", "Human", "Fork", "Join", "Subflow", "Decision", "Compensation"
	SystemOwned  bool           `json:"systemOwned"`
	ResourceKey  string         `json:"resourceKey,omitempty"`
	CapabilityID string         `json:"capabilityId,omitempty"`
	Timeout      time.Duration  `json:"timeout,omitempty"`
	Transitions  []string       `json:"transitions,omitempty"`
	Properties   map[string]any `json:"properties,omitempty"`
}

type ADGOPlan struct {
	WorkflowID   string     `json:"workflowId"`
	EntryNode    string     `json:"entryNode"`
	Nodes        []ADGONode `json:"nodes"`
	ResourceKeys []string   `json:"resourceKeys,omitempty"`
}

type Manifest struct {
	ScenarioID           string                `json:"scenarioId"`
	RevisionID           string                `json:"revisionId"`
	SemanticDigest       string                `json:"semanticDigest"`
	CompilerVersion      string                `json:"compilerVersion"`
	CapabilityVersions   map[string]string     `json:"capabilityVersions"`
	SelectedRuntime      Runtime               `json:"selectedRuntime"`
	RuntimeReasons       []string              `json:"runtimeReasons"`
	PlanDigest           string                `json:"planDigest"`
	RequiredPermissions  []string              `json:"requiredPermissions"`
	ReferencedEntities   []model.EntityRef     `json:"referencedEntities"`
	PhysicalResources    []string              `json:"physicalResources"`
	ExternalEffects      []ExternalEffectSpec  `json:"externalEffects"`
	SafetyAugmentations  []SafetyAugmentation  `json:"safetyAugmentations"`
	TemporalRequirements []TemporalRequirement `json:"temporalRequirements"`
	Migration            MigrationInfo         `json:"migration"`
	UserGraph            GraphRepresentation   `json:"userGraph"`
	SystemGraph          GraphRepresentation   `json:"systemGraph"`
	AxiomPlan            *AxiomPlan            `json:"axiomPlan,omitempty"`
	ADGOPlan             *ADGOPlan             `json:"adgoPlan,omitempty"`
	Diagnostics          Diagnostics           `json:"diagnostics"`
}
