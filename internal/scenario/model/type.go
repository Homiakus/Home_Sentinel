package model

import (
	"fmt"
	"sort"
	"strings"
)

type ValueKind string

const (
	TypeBool        ValueKind = "bool"
	TypeString      ValueKind = "string"
	TypeInt         ValueKind = "int"
	TypeFloat       ValueKind = "float"
	TypeDuration    ValueKind = "duration"
	TypeTimeOfDay   ValueKind = "time_of_day"
	TypeTimestamp   ValueKind = "timestamp"
	TypePercentage  ValueKind = "percentage"
	TypeConfidence  ValueKind = "confidence"
	TypeTemperature ValueKind = "temperature"
	TypeIlluminance ValueKind = "illuminance"
	TypeDistance    ValueKind = "distance"
	TypeEnum        ValueKind = "enum"
	TypeEntityRef   ValueKind = "entity_ref"
	TypeArtifactRef ValueKind = "artifact_ref"
	TypeList        ValueKind = "list"
)

type TypeRef struct {
	Kind       ValueKind `json:"kind"`
	Unit       string    `json:"unit,omitempty"`
	Name       string    `json:"name,omitempty"`
	EntityKind string    `json:"entityKind,omitempty"`
	Enum       []string  `json:"enum,omitempty"`
	Element    *TypeRef  `json:"element,omitempty"`
}

func (t TypeRef) Normalize() (TypeRef, error) {
	t.Unit = strings.TrimSpace(strings.ToLower(t.Unit))
	t.Name = strings.TrimSpace(t.Name)
	t.EntityKind = strings.TrimSpace(t.EntityKind)
	if len(t.Enum) > 0 {
		values := make([]string, 0, len(t.Enum))
		seen := map[string]struct{}{}
		for _, value := range t.Enum {
			value = strings.TrimSpace(value)
			if value == "" {
				return TypeRef{}, fmt.Errorf("scenario: enum contains empty value")
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
		sort.Strings(values)
		t.Enum = values
	}
	if t.Element != nil {
		element, err := t.Element.Normalize()
		if err != nil {
			return TypeRef{}, err
		}
		t.Element = &element
	}
	if err := t.Validate(); err != nil {
		return TypeRef{}, err
	}
	return t, nil
}

func (t TypeRef) Validate() error {
	switch t.Kind {
	case TypeBool, TypeString, TypeInt, TypeFloat, TypeDuration, TypeTimeOfDay, TypeTimestamp,
		TypePercentage, TypeConfidence, TypeTemperature, TypeIlluminance, TypeDistance, TypeArtifactRef:
		if t.Name != "" || t.Element != nil || len(t.Enum) != 0 || t.EntityKind != "" {
			return fmt.Errorf("scenario: type %q has incompatible metadata", t.Kind)
		}
	case TypeEnum:
		if t.Name == "" || len(t.Enum) == 0 || t.Element != nil || t.EntityKind != "" {
			return fmt.Errorf("scenario: enum type requires name and values")
		}
	case TypeEntityRef:
		if t.Name != "" || t.EntityKind == "" || t.Element != nil || len(t.Enum) != 0 {
			return fmt.Errorf("scenario: entity_ref requires entityKind")
		}
	case TypeList:
		if t.Name != "" || t.Element == nil || len(t.Enum) != 0 || t.EntityKind != "" {
			return fmt.Errorf("scenario: list requires element type")
		}
		if err := t.Element.Validate(); err != nil {
			return fmt.Errorf("scenario: list element: %w", err)
		}
	default:
		return fmt.Errorf("scenario: unknown value type %q", t.Kind)
	}
	canonical, quantitative := canonicalUnit(t.Kind)
	if quantitative {
		if t.Unit != canonical {
			return fmt.Errorf("scenario: type %q must use canonical unit %q", t.Kind, canonical)
		}
	} else if t.Unit != "" {
		return fmt.Errorf("scenario: type %q cannot declare unit %q", t.Kind, t.Unit)
	}
	return nil
}

func canonicalUnit(kind ValueKind) (string, bool) {
	switch kind {
	case TypeDuration:
		return "ns", true
	case TypePercentage, TypeConfidence:
		return "ratio", true
	case TypeTemperature:
		return "celsius", true
	case TypeIlluminance:
		return "lux", true
	case TypeDistance:
		return "meter", true
	default:
		return "", false
	}
}

func (t TypeRef) Compatible(other TypeRef) bool {
	left, err := t.Normalize()
	if err != nil {
		return false
	}
	right, err := other.Normalize()
	if err != nil {
		return false
	}
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case TypeEnum:
		if left.Name != right.Name || len(left.Enum) != len(right.Enum) {
			return false
		}
		for i := range left.Enum {
			if left.Enum[i] != right.Enum[i] {
				return false
			}
		}
	case TypeEntityRef:
		return left.EntityKind == right.EntityKind
	case TypeList:
		return left.Element.Compatible(*right.Element)
	}
	return left.Unit == right.Unit
}

func (t TypeRef) Ordered() bool {
	switch t.Kind {
	case TypeInt, TypeFloat, TypeDuration, TypeTimeOfDay, TypeTimestamp, TypePercentage, TypeConfidence, TypeTemperature, TypeIlluminance, TypeDistance, TypeString:
		return true
	default:
		return false
	}
}

func (t TypeRef) Numeric() bool {
	switch t.Kind {
	case TypeInt, TypeFloat, TypePercentage, TypeConfidence, TypeTemperature, TypeIlluminance, TypeDistance:
		return true
	default:
		return false
	}
}
