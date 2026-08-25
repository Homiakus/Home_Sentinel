package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Value is a capability-schema value. It is generic at Stage 28; Stage 30 adds
// the full typed/unit-aware value system. It must never contain provider-native
// request payloads or secrets.
type Value json.RawMessage

func ValueOf(v any) (Value, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return Value(raw), nil
}

func (v Value) MarshalJSON() ([]byte, error) {
	if len(v) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid(v) {
		return nil, fmt.Errorf("scenario: invalid value JSON")
	}
	return append([]byte(nil), v...), nil
}

func (v *Value) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("scenario: invalid value JSON")
	}
	*v = append((*v)[:0], data...)
	return nil
}

func (v Value) canonical() (Value, error) {
	if len(v) == 0 {
		return Value([]byte("null")), nil
	}
	dec := json.NewDecoder(bytes.NewReader(v))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("scenario: value contains multiple JSON values")
		}
		return nil, err
	}
	raw, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return Value(raw), nil
}

type Parameter struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Default  *Value `json:"default,omitempty"`
}

func (p Parameter) Validate() error {
	if err := validateToken("parameter id", p.ID); err != nil {
		return err
	}
	if err := validateToken("parameter type", p.Type); err != nil {
		return err
	}
	if p.Default != nil {
		if _, err := p.Default.canonical(); err != nil {
			return fmt.Errorf("scenario: parameter %q default: %w", p.ID, err)
		}
	}
	return nil
}
