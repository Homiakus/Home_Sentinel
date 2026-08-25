package model

import "fmt"

type Flow struct {
	Steps []Step `json:"steps"`
}

func (f Flow) Validate() error {
	if len(f.Steps) == 0 {
		return fmt.Errorf("scenario: flow must contain at least one step")
	}
	for i := range f.Steps {
		if err := f.Steps[i].Validate(); err != nil {
			return fmt.Errorf("scenario: step[%d]: %w", i, err)
		}
	}
	return nil
}

func normalizeFlow(f Flow) (Flow, error) {
	for i := range f.Steps {
		normalized, err := normalizeStep(f.Steps[i])
		if err != nil {
			return Flow{}, err
		}
		f.Steps[i] = normalized
	}
	if err := f.Validate(); err != nil {
		return Flow{}, err
	}
	return f, nil
}
