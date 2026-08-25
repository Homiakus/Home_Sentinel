package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

type Value struct {
	Type TypeRef         `json:"type"`
	Data json.RawMessage `json:"value"`
}

type ArtifactValue struct {
	URI       string `json:"uri"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType,omitempty"`
}

type EntityValue struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

func ValueOf(v any) (Value, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return Value{}, err
	}
	typ, err := inferType(raw)
	if err != nil {
		return Value{}, err
	}
	return NewTypedValue(typ, v)
}

func NewTypedValue(typ TypeRef, v any) (Value, error) {
	typ, err := typ.Normalize()
	if err != nil {
		return Value{}, err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return Value{}, err
	}
	return (Value{Type: typ, Data: raw}).canonical()
}

func NewDurationValue(duration time.Duration) (Value, error) {
	if duration < 0 {
		return Value{}, fmt.Errorf("scenario: duration cannot be negative")
	}
	return NewTypedValue(TypeRef{Kind: TypeDuration, Unit: "ns"}, duration.Nanoseconds())
}

func NewTimeOfDayValue(value string) (Value, error) {
	parsed, err := ParseTimeOfDay(value)
	if err != nil {
		return Value{}, err
	}
	return NewTypedValue(TypeRef{Kind: TypeTimeOfDay}, parsed.String())
}

func NewTimestampValue(value time.Time) (Value, error) {
	if value.IsZero() {
		return Value{}, fmt.Errorf("scenario: timestamp is required")
	}
	return NewTypedValue(TypeRef{Kind: TypeTimestamp}, value.UTC().Format(time.RFC3339Nano))
}

func NewQuantity(kind ValueKind, value float64, unit string) (Value, error) {
	canonical, ok := canonicalUnit(kind)
	if !ok || kind == TypeDuration {
		return Value{}, fmt.Errorf("scenario: %q is not a supported scalar quantity", kind)
	}
	converted, err := convertToCanonical(kind, value, unit)
	if err != nil {
		return Value{}, err
	}
	return NewTypedValue(TypeRef{Kind: kind, Unit: canonical}, converted)
}

func NewEnumValue(name string, allowed []string, value string) (Value, error) {
	return NewTypedValue(TypeRef{Kind: TypeEnum, Name: name, Enum: allowed}, value)
}

func NewEntityRefValue(kind, id string) (Value, error) {
	if strings.TrimSpace(kind) == "" || strings.TrimSpace(id) == "" {
		return Value{}, fmt.Errorf("scenario: entity kind and id are required")
	}
	return NewTypedValue(TypeRef{Kind: TypeEntityRef, EntityKind: strings.TrimSpace(kind)}, EntityValue{ID: strings.TrimSpace(id), Kind: strings.TrimSpace(kind)})
}

func NewArtifactRefValue(value ArtifactValue) (Value, error) {
	return NewTypedValue(TypeRef{Kind: TypeArtifactRef}, value)
}

func (v Value) MarshalJSON() ([]byte, error) {
	canonical, err := v.canonical()
	if err != nil {
		return nil, err
	}
	type wire Value
	return json.Marshal(wire(canonical))
}

func (v *Value) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("scenario: invalid value JSON")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err == nil {
		if _, typed := probe["type"]; typed {
			type wire Value
			var decoded wire
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&decoded); err != nil {
				return fmt.Errorf("scenario: decode typed value: %w", err)
			}
			canonical, err := Value(decoded).canonical()
			if err != nil {
				return err
			}
			*v = canonical
			return nil
		}
	}
	// Stage 28 compatibility: raw scalars/lists are accepted on read and become
	// typed immediately. Canonical serialization always emits type + value.
	typ, err := inferType(data)
	if err != nil {
		return err
	}
	canonical, err := (Value{Type: typ, Data: append(json.RawMessage(nil), data...)}).canonical()
	if err != nil {
		return err
	}
	*v = canonical
	return nil
}

func (v Value) canonical() (Value, error) {
	typ, err := v.Type.Normalize()
	if err != nil {
		return Value{}, err
	}
	if len(v.Data) == 0 || !json.Valid(v.Data) {
		return Value{}, fmt.Errorf("scenario: typed value contains invalid JSON")
	}
	data, err := canonicalData(typ, v.Data)
	if err != nil {
		return Value{}, err
	}
	return Value{Type: typ, Data: data}, nil
}

func inferType(raw []byte) (TypeRef, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return TypeRef{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return TypeRef{}, fmt.Errorf("scenario: value contains trailing JSON")
	}
	return inferDecodedType(value)
}

func inferDecodedType(value any) (TypeRef, error) {
	switch value := value.(type) {
	case bool:
		return TypeRef{Kind: TypeBool}, nil
	case string:
		return TypeRef{Kind: TypeString}, nil
	case json.Number:
		if strings.ContainsAny(value.String(), ".eE") {
			if _, err := value.Float64(); err != nil {
				return TypeRef{}, err
			}
			return TypeRef{Kind: TypeFloat}, nil
		}
		if _, err := value.Int64(); err != nil {
			return TypeRef{}, err
		}
		return TypeRef{Kind: TypeInt}, nil
	case []any:
		if len(value) == 0 {
			return TypeRef{}, fmt.Errorf("scenario: empty list requires an explicit element type")
		}
		element, err := inferDecodedType(value[0])
		if err != nil {
			return TypeRef{}, err
		}
		for i := 1; i < len(value); i++ {
			candidate, err := inferDecodedType(value[i])
			if err != nil || !element.Compatible(candidate) {
				return TypeRef{}, fmt.Errorf("scenario: list elements must share one type")
			}
		}
		return TypeRef{Kind: TypeList, Element: &element}, nil
	default:
		return TypeRef{}, fmt.Errorf("scenario: cannot infer typed value from %T; use NewTypedValue", value)
	}
}

func canonicalData(typ TypeRef, raw json.RawMessage) (json.RawMessage, error) {
	switch typ.Kind {
	case TypeBool:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, typeDataError(typ, err)
		}
		return json.Marshal(value)
	case TypeString:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, typeDataError(typ, err)
		}
		return json.Marshal(value)
	case TypeInt, TypeDuration:
		var number json.Number
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&number); err != nil {
			return nil, typeDataError(typ, err)
		}
		value, err := number.Int64()
		if err != nil {
			return nil, typeDataError(typ, err)
		}
		if typ.Kind == TypeDuration && value < 0 {
			return nil, fmt.Errorf("scenario: duration cannot be negative")
		}
		return json.Marshal(value)
	case TypeFloat, TypePercentage, TypeConfidence, TypeTemperature, TypeIlluminance, TypeDistance:
		var number json.Number
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&number); err != nil {
			return nil, typeDataError(typ, err)
		}
		value, err := number.Float64()
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("scenario: invalid %s number", typ.Kind)
		}
		if (typ.Kind == TypePercentage || typ.Kind == TypeConfidence) && (value < 0 || value > 1) {
			return nil, fmt.Errorf("scenario: %s must be in [0,1]", typ.Kind)
		}
		if typ.Kind == TypeTemperature && value < -273.15 {
			return nil, fmt.Errorf("scenario: temperature cannot be below absolute zero")
		}
		if typ.Kind == TypeIlluminance && value < 0 {
			return nil, fmt.Errorf("scenario: illuminance cannot be negative")
		}
		if typ.Kind == TypeDistance && value < 0 {
			return nil, fmt.Errorf("scenario: distance cannot be negative")
		}
		return json.Marshal(value)
	case TypeTimeOfDay:
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, typeDataError(typ, err)
		}
		parsed, err := ParseTimeOfDay(text)
		if err != nil {
			return nil, err
		}
		return json.Marshal(parsed.String())
	case TypeTimestamp:
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, typeDataError(typ, err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return nil, fmt.Errorf("scenario: invalid timestamp: %w", err)
		}
		return json.Marshal(parsed.UTC().Format(time.RFC3339Nano))
	case TypeEnum:
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, typeDataError(typ, err)
		}
		for _, allowed := range typ.Enum {
			if text == allowed {
				return json.Marshal(text)
			}
		}
		return nil, fmt.Errorf("scenario: enum %q does not allow %q", typ.Name, text)
	case TypeEntityRef:
		var entity EntityValue
		if err := strictDecode(raw, &entity); err != nil {
			return nil, typeDataError(typ, err)
		}
		entity.ID = strings.TrimSpace(entity.ID)
		entity.Kind = strings.TrimSpace(entity.Kind)
		if entity.ID == "" || entity.Kind != typ.EntityKind {
			return nil, fmt.Errorf("scenario: entity ref must target kind %q with non-empty id", typ.EntityKind)
		}
		return json.Marshal(entity)
	case TypeArtifactRef:
		var artifact ArtifactValue
		if err := strictDecode(raw, &artifact); err != nil {
			return nil, typeDataError(typ, err)
		}
		artifact.URI = strings.TrimSpace(artifact.URI)
		artifact.Digest = strings.TrimSpace(artifact.Digest)
		artifact.MediaType = strings.TrimSpace(artifact.MediaType)
		if artifact.URI == "" || artifact.Digest == "" || artifact.Size < 0 {
			return nil, fmt.Errorf("scenario: invalid artifact ref")
		}
		return json.Marshal(artifact)
	case TypeList:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, typeDataError(typ, err)
		}
		canonical := make([]json.RawMessage, len(items))
		for i := range items {
			item, err := canonicalData(*typ.Element, items[i])
			if err != nil {
				return nil, fmt.Errorf("scenario: list item %d: %w", i, err)
			}
			canonical[i] = item
		}
		return json.Marshal(canonical)
	default:
		return nil, fmt.Errorf("scenario: unsupported value type %q", typ.Kind)
	}
}

func typeDataError(typ TypeRef, err error) error {
	return fmt.Errorf("scenario: value is not %s: %w", typ.Kind, err)
}

func strictDecode(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func convertToCanonical(kind ValueKind, value float64, unit string) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("scenario: quantity must be finite")
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch kind {
	case TypePercentage, TypeConfidence:
		switch unit {
		case "ratio", "":
			return boundedRatio(kind, value)
		case "%", "percent":
			return boundedRatio(kind, value/100)
		}
	case TypeTemperature:
		switch unit {
		case "c", "°c", "celsius":
			return value, nil
		case "f", "°f", "fahrenheit":
			return (value - 32) * 5 / 9, nil
		case "k", "kelvin":
			return value - 273.15, nil
		}
	case TypeIlluminance:
		if unit == "lux" || unit == "lx" {
			if value < 0 {
				return 0, fmt.Errorf("scenario: illuminance cannot be negative")
			}
			return value, nil
		}
	case TypeDistance:
		var factor float64
		switch unit {
		case "m", "meter", "meters":
			factor = 1
		case "cm":
			factor = .01
		case "mm":
			factor = .001
		case "km":
			factor = 1000
		case "ft", "feet":
			factor = .3048
		default:
			return 0, fmt.Errorf("scenario: unsupported distance unit %q", unit)
		}
		if value < 0 {
			return 0, fmt.Errorf("scenario: distance cannot be negative")
		}
		return value * factor, nil
	}
	return 0, fmt.Errorf("scenario: unsupported unit %q for %q", unit, kind)
}

func boundedRatio(kind ValueKind, value float64) (float64, error) {
	if value < 0 || value > 1 {
		return 0, fmt.Errorf("scenario: %s must be in [0,1]", kind)
	}
	return value, nil
}

func numberValue(v Value) (float64, error) {
	canonical, err := v.canonical()
	if err != nil {
		return 0, err
	}
	var number float64
	if err := json.Unmarshal(canonical.Data, &number); err != nil {
		return 0, err
	}
	return number, nil
}

type Parameter struct {
	ID       string  `json:"id"`
	Type     TypeRef `json:"type"`
	Required bool    `json:"required,omitempty"`
	Default  *Value  `json:"default,omitempty"`
}

func (p Parameter) Validate() error {
	if err := validateToken("parameter id", p.ID); err != nil {
		return err
	}
	typ, err := p.Type.Normalize()
	if err != nil {
		return fmt.Errorf("scenario: parameter %q type: %w", p.ID, err)
	}
	if p.Default != nil {
		value, err := p.Default.canonical()
		if err != nil {
			return fmt.Errorf("scenario: parameter %q default: %w", p.ID, err)
		}
		if !typ.Compatible(value.Type) {
			return fmt.Errorf("scenario: parameter %q default type %q does not match %q", p.ID, value.Type.Kind, typ.Kind)
		}
	}
	return nil
}
