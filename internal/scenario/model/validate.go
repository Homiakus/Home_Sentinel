package model

import (
	"fmt"
	"strings"
)

func (s Scenario) Validate() error {
	if err := validateToken("scenario id", string(s.ID)); err != nil {
		return err
	}
	if err := validateToken("revision id", string(s.RevisionID)); err != nil {
		return err
	}
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return fmt.Errorf("scenario: name is required")
	}
	if len(name) > 200 {
		return fmt.Errorf("scenario: name exceeds 200 bytes")
	}
	if len(s.Description) > 4000 {
		return fmt.Errorf("scenario: description exceeds 4000 bytes")
	}
	if len(s.Triggers) == 0 {
		return fmt.Errorf("scenario: at least one trigger is required")
	}
	triggerIDs := make(map[string]struct{}, len(s.Triggers))
	for i := range s.Triggers {
		if err := s.Triggers[i].Validate(); err != nil {
			return fmt.Errorf("scenario: trigger[%d]: %w", i, err)
		}
		if _, exists := triggerIDs[s.Triggers[i].ID]; exists {
			return fmt.Errorf("scenario: duplicate trigger id %q", s.Triggers[i].ID)
		}
		triggerIDs[s.Triggers[i].ID] = struct{}{}
	}
	if !s.Condition.IsZero() {
		if err := s.Condition.Validate(); err != nil {
			return fmt.Errorf("scenario: condition: %w", err)
		}
	}
	if err := s.Flow.Validate(); err != nil {
		return err
	}
	stepIDs := map[string]struct{}{}
	if err := collectFlowStepIDs(s.Flow, stepIDs); err != nil {
		return err
	}
	parameterIDs := make(map[string]struct{}, len(s.Parameters))
	for i := range s.Parameters {
		if err := s.Parameters[i].Validate(); err != nil {
			return fmt.Errorf("scenario: parameter[%d]: %w", i, err)
		}
		if _, exists := parameterIDs[s.Parameters[i].ID]; exists {
			return fmt.Errorf("scenario: duplicate parameter id %q", s.Parameters[i].ID)
		}
		parameterIDs[s.Parameters[i].ID] = struct{}{}
	}
	for stepID := range s.Metadata.Layout {
		if _, exists := stepIDs[stepID]; !exists {
			return fmt.Errorf("scenario: layout references unknown step %q", stepID)
		}
	}
	return nil
}

func collectFlowStepIDs(flow Flow, seen map[string]struct{}) error {
	for i := range flow.Steps {
		step := flow.Steps[i]
		id := string(step.ID)
		if _, exists := seen[id]; exists {
			return fmt.Errorf("scenario: duplicate step id %q", id)
		}
		seen[id] = struct{}{}
		if step.If != nil {
			if err := collectFlowStepIDs(step.If.Then, seen); err != nil {
				return err
			}
			if step.If.Else != nil {
				if err := collectFlowStepIDs(*step.If.Else, seen); err != nil {
					return err
				}
			}
		}
		if step.Switch != nil {
			for j := range step.Switch.Cases {
				if err := collectFlowStepIDs(step.Switch.Cases[j].Flow, seen); err != nil {
					return err
				}
			}
			if step.Switch.Default != nil {
				if err := collectFlowStepIDs(*step.Switch.Default, seen); err != nil {
					return err
				}
			}
		}
		if step.Parallel != nil {
			for j := range step.Parallel.Branches {
				if err := collectFlowStepIDs(step.Parallel.Branches[j], seen); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
