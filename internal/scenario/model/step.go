package model

import (
	"fmt"
	"strings"
	"time"
)

type StepKind string

const (
	StepAction        StepKind = "action"
	StepWait          StepKind = "wait"
	StepIf            StepKind = "if"
	StepSwitch        StepKind = "switch"
	StepParallel      StepKind = "parallel"
	StepJoin          StepKind = "join"
	StepHumanApproval StepKind = "human_approval"
	StepSubflow       StepKind = "subflow"
	StepStop          StepKind = "stop"
)

type RetryHint struct {
	MaxAttempts int           `json:"maxAttempts"`
	BaseDelay   time.Duration `json:"baseDelay,omitempty"`
	MaxDelay    time.Duration `json:"maxDelay,omitempty"`
}

func (r RetryHint) Validate() error {
	if r.MaxAttempts < 1 || r.MaxAttempts > 10 {
		return fmt.Errorf("scenario: retry maxAttempts must be between 1 and 10")
	}
	if r.BaseDelay < 0 || r.MaxDelay < 0 {
		return fmt.Errorf("scenario: retry delays must be non-negative")
	}
	if r.MaxDelay > 0 && r.BaseDelay > r.MaxDelay {
		return fmt.Errorf("scenario: retry baseDelay cannot exceed maxDelay")
	}
	return nil
}

type ActionStep struct {
	Capability CapabilityRef    `json:"capability"`
	Arguments  map[string]Value `json:"arguments,omitempty"`
	Retry      *RetryHint       `json:"retry,omitempty"`
}

type WaitStep struct {
	Duration time.Duration `json:"duration"`
}

type IfStep struct {
	Condition Expr  `json:"condition"`
	Then      Flow  `json:"then"`
	Else      *Flow `json:"else,omitempty"`
}

type SwitchCase struct {
	When Expr `json:"when"`
	Flow Flow `json:"flow"`
}

type SwitchStep struct {
	Cases   []SwitchCase `json:"cases"`
	Default *Flow        `json:"default,omitempty"`
}

type ParallelStep struct {
	Branches []Flow `json:"branches"`
}

type JoinMode string

const (
	JoinAll    JoinMode = "all"
	JoinAny    JoinMode = "any"
	JoinNOfM   JoinMode = "n_of_m"
	JoinQuorum JoinMode = "quorum"
)

type JoinStep struct {
	Mode      JoinMode `json:"mode"`
	Threshold int      `json:"threshold,omitempty"`
}

type HumanApprovalStep struct {
	Risk    RiskLevel     `json:"risk"`
	Prompt  string        `json:"prompt"`
	Timeout time.Duration `json:"timeout,omitempty"`
}

type SubflowStep struct {
	ScenarioID ID               `json:"scenarioId"`
	Version    Version          `json:"version"`
	Arguments  map[string]Value `json:"arguments,omitempty"`
}

type StopOutcome string

const (
	StopCompleted StopOutcome = "completed"
	StopCanceled  StopOutcome = "canceled"
	StopRejected  StopOutcome = "rejected"
)

type StopStep struct {
	Outcome StopOutcome `json:"outcome"`
}

type Step struct {
	ID            StepID             `json:"id"`
	Kind          StepKind           `json:"kind"`
	Action        *ActionStep        `json:"action,omitempty"`
	Wait          *WaitStep          `json:"wait,omitempty"`
	If            *IfStep            `json:"if,omitempty"`
	Switch        *SwitchStep        `json:"switch,omitempty"`
	Parallel      *ParallelStep      `json:"parallel,omitempty"`
	Join          *JoinStep          `json:"join,omitempty"`
	HumanApproval *HumanApprovalStep `json:"humanApproval,omitempty"`
	Subflow       *SubflowStep       `json:"subflow,omitempty"`
	Stop          *StopStep          `json:"stop,omitempty"`
}

func (s Step) Validate() error {
	if err := validateToken("step id", string(s.ID)); err != nil {
		return err
	}
	if s.payloadCount() != 1 {
		return fmt.Errorf("scenario: step %q must contain exactly one payload", s.ID)
	}
	switch s.Kind {
	case StepAction:
		if s.Action == nil {
			return wrongPayload(s)
		}
		return validateAction(*s.Action)
	case StepWait:
		if s.Wait == nil || s.Wait.Duration <= 0 {
			return fmt.Errorf("scenario: wait step %q requires positive duration", s.ID)
		}
	case StepIf:
		if s.If == nil {
			return wrongPayload(s)
		}
		if s.If.Condition.IsZero() {
			return fmt.Errorf("scenario: if step %q requires condition", s.ID)
		}
		if err := s.If.Condition.Validate(); err != nil {
			return err
		}
		if err := s.If.Then.Validate(); err != nil {
			return err
		}
		if s.If.Else != nil {
			if err := s.If.Else.Validate(); err != nil {
				return err
			}
		}
	case StepSwitch:
		if s.Switch == nil || len(s.Switch.Cases) == 0 {
			return fmt.Errorf("scenario: switch step %q requires at least one case", s.ID)
		}
		for i := range s.Switch.Cases {
			if s.Switch.Cases[i].When.IsZero() {
				return fmt.Errorf("scenario: switch step %q case %d requires condition", s.ID, i)
			}
			if err := s.Switch.Cases[i].When.Validate(); err != nil {
				return err
			}
			if err := s.Switch.Cases[i].Flow.Validate(); err != nil {
				return err
			}
		}
		if s.Switch.Default != nil {
			if err := s.Switch.Default.Validate(); err != nil {
				return err
			}
		}
	case StepParallel:
		if s.Parallel == nil || len(s.Parallel.Branches) < 2 {
			return fmt.Errorf("scenario: parallel step %q requires at least two branches", s.ID)
		}
		for i := range s.Parallel.Branches {
			if err := s.Parallel.Branches[i].Validate(); err != nil {
				return fmt.Errorf("scenario: parallel step %q branch %d: %w", s.ID, i, err)
			}
		}
	case StepJoin:
		if s.Join == nil {
			return wrongPayload(s)
		}
		if err := validateJoin(*s.Join); err != nil {
			return err
		}
	case StepHumanApproval:
		if s.HumanApproval == nil {
			return wrongPayload(s)
		}
		if err := s.HumanApproval.Risk.Validate(); err != nil {
			return err
		}
		if strings.TrimSpace(s.HumanApproval.Prompt) == "" {
			return fmt.Errorf("scenario: human approval step %q requires prompt", s.ID)
		}
		if s.HumanApproval.Timeout < 0 {
			return fmt.Errorf("scenario: human approval step %q timeout cannot be negative", s.ID)
		}
	case StepSubflow:
		if s.Subflow == nil {
			return wrongPayload(s)
		}
		if err := validateToken("subflow scenario id", string(s.Subflow.ScenarioID)); err != nil {
			return err
		}
		if s.Subflow.Version == 0 {
			return fmt.Errorf("scenario: subflow step %q must reference a published version", s.ID)
		}
		if err := validateValues("subflow argument", s.Subflow.Arguments); err != nil {
			return err
		}
	case StepStop:
		if s.Stop == nil {
			return wrongPayload(s)
		}
		switch s.Stop.Outcome {
		case StopCompleted, StopCanceled, StopRejected:
		default:
			return fmt.Errorf("scenario: stop step %q has invalid outcome %q", s.ID, s.Stop.Outcome)
		}
	default:
		return fmt.Errorf("scenario: unknown step kind %q", s.Kind)
	}
	return nil
}

func validateAction(action ActionStep) error {
	if err := action.Capability.Validate(); err != nil {
		return err
	}
	if err := validateValues("action argument", action.Arguments); err != nil {
		return err
	}
	if action.Retry != nil {
		return action.Retry.Validate()
	}
	return nil
}

func validateValues(label string, values map[string]Value) error {
	for key, value := range values {
		if err := validateToken(label, key); err != nil {
			return err
		}
		if _, err := value.canonical(); err != nil {
			return fmt.Errorf("scenario: %s %q: %w", label, key, err)
		}
	}
	return nil
}

func validateJoin(join JoinStep) error {
	switch join.Mode {
	case JoinAll, JoinAny:
		if join.Threshold != 0 {
			return fmt.Errorf("scenario: join mode %q does not accept threshold", join.Mode)
		}
	case JoinNOfM, JoinQuorum:
		if join.Threshold <= 0 {
			return fmt.Errorf("scenario: join mode %q requires positive threshold", join.Mode)
		}
	default:
		return fmt.Errorf("scenario: invalid join mode %q", join.Mode)
	}
	return nil
}

func (s Step) payloadCount() int {
	count := 0
	for _, present := range []bool{s.Action != nil, s.Wait != nil, s.If != nil, s.Switch != nil, s.Parallel != nil, s.Join != nil, s.HumanApproval != nil, s.Subflow != nil, s.Stop != nil} {
		if present {
			count++
		}
	}
	return count
}

func wrongPayload(s Step) error {
	return fmt.Errorf("scenario: step %q kind %q has mismatched payload", s.ID, s.Kind)
}

func normalizeStep(s Step) (Step, error) {
	s.ID = StepID(strings.TrimSpace(string(s.ID)))
	if s.Action != nil {
		normalizeCapability(&s.Action.Capability)
		if err := normalizeValues(s.Action.Arguments); err != nil {
			return Step{}, err
		}
	}
	if s.If != nil {
		condition, err := normalizeExpr(s.If.Condition)
		if err != nil {
			return Step{}, err
		}
		s.If.Condition = condition
		thenFlow, err := normalizeFlow(s.If.Then)
		if err != nil {
			return Step{}, err
		}
		s.If.Then = thenFlow
		if s.If.Else != nil {
			elseFlow, err := normalizeFlow(*s.If.Else)
			if err != nil {
				return Step{}, err
			}
			s.If.Else = &elseFlow
		}
	}
	if s.Switch != nil {
		for i := range s.Switch.Cases {
			when, err := normalizeExpr(s.Switch.Cases[i].When)
			if err != nil {
				return Step{}, err
			}
			s.Switch.Cases[i].When = when
			flow, err := normalizeFlow(s.Switch.Cases[i].Flow)
			if err != nil {
				return Step{}, err
			}
			s.Switch.Cases[i].Flow = flow
		}
		if s.Switch.Default != nil {
			flow, err := normalizeFlow(*s.Switch.Default)
			if err != nil {
				return Step{}, err
			}
			s.Switch.Default = &flow
		}
	}
	if s.Parallel != nil {
		for i := range s.Parallel.Branches {
			flow, err := normalizeFlow(s.Parallel.Branches[i])
			if err != nil {
				return Step{}, err
			}
			s.Parallel.Branches[i] = flow
		}
	}
	if s.HumanApproval != nil {
		s.HumanApproval.Prompt = strings.TrimSpace(s.HumanApproval.Prompt)
	}
	if s.Subflow != nil {
		s.Subflow.ScenarioID = ID(strings.TrimSpace(string(s.Subflow.ScenarioID)))
		if err := normalizeValues(s.Subflow.Arguments); err != nil {
			return Step{}, err
		}
	}
	if err := s.Validate(); err != nil {
		return Step{}, err
	}
	return s, nil
}

func normalizeCapability(ref *CapabilityRef) {
	ref.ID = strings.TrimSpace(ref.ID)
	ref.Version = strings.TrimSpace(ref.Version)
	if ref.Entity != nil {
		ref.Entity.ID = strings.TrimSpace(ref.Entity.ID)
		ref.Entity.Kind = strings.TrimSpace(ref.Entity.Kind)
	}
}

func normalizeValues(values map[string]Value) error {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]Value, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if err := validateToken("value key", key); err != nil {
			return err
		}
		if _, exists := normalized[key]; exists {
			return fmt.Errorf("scenario: duplicate normalized value key %q", key)
		}
		canonical, err := value.canonical()
		if err != nil {
			return err
		}
		normalized[key] = canonical
	}
	for key := range values {
		delete(values, key)
	}
	for key, value := range normalized {
		values[key] = value
	}
	return nil
}
