package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const MaxScenarioBytes = 1 << 20

type Scenario struct {
	ID          ID         `json:"id"`
	RevisionID  RevisionID `json:"revisionId"`
	Version     Version    `json:"version"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Enabled     bool       `json:"enabled"`

	Triggers  []Trigger `json:"triggers"`
	Condition Expr      `json:"condition"`
	Flow      Flow      `json:"flow"`

	Parameters []Parameter `json:"parameters,omitempty"`
	Metadata   Metadata    `json:"metadata,omitempty"`
}

func DecodeScenario(data []byte) (Scenario, error) {
	if len(data) > MaxScenarioBytes {
		return Scenario{}, fmt.Errorf("scenario: document exceeds %d bytes", MaxScenarioBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var scenario Scenario
	if err := dec.Decode(&scenario); err != nil {
		return Scenario{}, fmt.Errorf("scenario: decode: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Scenario{}, fmt.Errorf("scenario: multiple JSON documents are not allowed")
		}
		return Scenario{}, fmt.Errorf("scenario: decode trailing data: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}

func Clone(source Scenario) (Scenario, error) {
	if err := source.Validate(); err != nil {
		return Scenario{}, err
	}
	clone, err := deepCopyScenario(source)
	if err != nil {
		return Scenario{}, err
	}
	id, err := NewID("scenario")
	if err != nil {
		return Scenario{}, err
	}
	revision, err := NewRevisionID()
	if err != nil {
		return Scenario{}, err
	}
	clone.ID = id
	clone.RevisionID = revision
	clone.Version = 0
	clone.Enabled = false
	return clone, clone.Validate()
}

func deepCopyScenario(source Scenario) (Scenario, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return Scenario{}, err
	}
	var clone Scenario
	if err := json.Unmarshal(raw, &clone); err != nil {
		return Scenario{}, err
	}
	return clone, nil
}
