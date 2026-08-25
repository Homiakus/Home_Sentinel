package capability

import (
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type FieldSchema struct {
	Name        string        `json:"name"`
	Type        model.TypeRef `json:"type"`
	Required    bool          `json:"required,omitempty"`
	Description string        `json:"description,omitempty"`
	Enum        []string      `json:"enum,omitempty"`
	Default     *model.Value  `json:"default,omitempty"`
}

type Schema struct {
	Fields []FieldSchema `json:"fields,omitempty"`
}

func (s Schema) Validate() error {
	seen := make(map[string]struct{}, len(s.Fields))
	for i, field := range s.Fields {
		if err := validateID("field name", field.Name); err != nil {
			return fmt.Errorf("capability: field[%d]: %w", i, err)
		}
		typ, err := field.Type.Normalize()
		if err != nil {
			return fmt.Errorf("capability: field %q type: %w", field.Name, err)
		}
		if _, exists := seen[field.Name]; exists {
			return fmt.Errorf("capability: duplicate field %q", field.Name)
		}
		seen[field.Name] = struct{}{}
		if len(field.Enum) > 0 && typ.Kind != model.TypeEnum {
			return fmt.Errorf("capability: field %q enum UI values require enum type", field.Name)
		}
		if field.Default != nil {
			raw, err := field.Default.MarshalJSON()
			if err != nil || len(raw) == 0 {
				return fmt.Errorf("capability: field %q has invalid default", field.Name)
			}
			if !typ.Compatible(field.Default.Type) {
				return fmt.Errorf("capability: field %q default type does not match schema type", field.Name)
			}
		}
	}
	return nil
}

func normalizeSchema(schema Schema) Schema {
	for i := range schema.Fields {
		schema.Fields[i].Name = strings.TrimSpace(schema.Fields[i].Name)
		schema.Fields[i].Description = strings.TrimSpace(schema.Fields[i].Description)
		if typ, err := schema.Fields[i].Type.Normalize(); err == nil {
			schema.Fields[i].Type = typ
		}
	}
	return schema
}
