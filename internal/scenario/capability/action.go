package capability

import (
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

func NewActionDescriptor(id, version, providerID, integrationID string, category Category, title string, permission Permission) (Descriptor, error) {
	return NewDescriptor(KindAction, id, version, providerID, integrationID, category, title, permission)
}

// ValidateActionArguments validates that the provided action arguments conform to the descriptor's input schema.
func ValidateActionArguments(desc Descriptor, args map[string]model.Expr, env model.TypeEnv) error {
	if desc.Kind != KindAction {
		return fmt.Errorf("capability: descriptor %q is not an action", desc.ID)
	}
	schemaFields := make(map[string]FieldSchema, len(desc.Input.Fields))
	for _, field := range desc.Input.Fields {
		schemaFields[field.Name] = field
	}

	for argName := range args {
		if _, exists := schemaFields[argName]; !exists {
			return fmt.Errorf("capability %q: unknown argument %q", desc.ID, argName)
		}
	}

	for _, field := range desc.Input.Fields {
		argExpr, exists := args[field.Name]
		if !exists || argExpr.IsZero() {
			if field.Required && field.Default == nil {
				return fmt.Errorf("capability %q: missing required argument %q", desc.ID, field.Name)
			}
			continue
		}

		inferredType, err := model.CheckExpr(argExpr, env)
		if err != nil {
			return fmt.Errorf("capability %q: argument %q: %w", desc.ID, field.Name, err)
		}

		if !field.Type.Compatible(inferredType) {
			return fmt.Errorf("capability %q: argument %q expected type %q, got %q", desc.ID, field.Name, field.Type.Kind, inferredType.Kind)
		}

		if len(field.Enum) > 0 && strings.ToLower(argExpr.Op) == "literal" && argExpr.Value != nil && argExpr.Value.Type.Kind == model.TypeString {
			var valStr string
			if err := argExpr.Value.Unmarshal(&valStr); err == nil {
				matched := false
				for _, allowed := range field.Enum {
					if allowed == valStr {
						matched = true
						break
					}
				}
				if !matched {
					return fmt.Errorf("capability %q: argument %q value %q is not allowed by enum %v", desc.ID, field.Name, valStr, field.Enum)
				}
			}
		}
	}

	return nil
}
