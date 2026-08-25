package capability

import (
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

func TestValidateActionArguments(t *testing.T) {
	titleVal, err := model.ValueOf("Alert")
	if err != nil {
		t.Fatalf("ValueOf title: %v", err)
	}
	priorityType := model.TypeRef{
		Kind: model.TypeEnum,
		Name: "priority",
		Enum: []string{"high", "low", "medium"},
	}
	priorityVal, err := model.NewEnumValue("priority", []string{"high", "low", "medium"}, "high")
	if err != nil {
		t.Fatalf("NewEnumValue priority: %v", err)
	}
	badPriorityVal, err := model.NewEnumValue("other_enum", []string{"custom"}, "custom")
	if err != nil {
		t.Fatalf("NewEnumValue badPriority: %v", err)
	}
	intVal, err := model.ValueOf(int64(123))
	if err != nil {
		t.Fatalf("ValueOf int: %v", err)
	}

	desc, err := NewActionDescriptor("notify.send", "1.0.0", "core", "notification", "alert", "Send Notification", "notification:send")
	if err != nil {
		t.Fatalf("NewActionDescriptor failed: %v", err)
	}
	desc.Input = Schema{
		Fields: []FieldSchema{
			{
				Name:     "title",
				Type:     model.TypeRef{Kind: model.TypeString},
				Required: true,
			},
			{
				Name: "priority",
				Type: priorityType,
				Enum: []string{"high", "low", "medium"},
			},
			{
				Name: "retry_count",
				Type: model.TypeRef{Kind: model.TypeInt},
			},
		},
	}
	normDesc, err := NormalizeDescriptor(desc)
	if err != nil {
		t.Fatalf("NormalizeDescriptor failed: %v", err)
	}

	env := model.TypeEnv{
		"trigger.title": model.TypeRef{Kind: model.TypeString},
	}

	// 1. Valid arguments
	validArgs := map[string]model.Expr{
		"title":       model.Ref("trigger.title"),
		"priority":    model.Literal(priorityVal),
		"retry_count": model.Literal(intVal),
	}
	if err := ValidateActionArguments(normDesc, validArgs, env); err != nil {
		t.Fatalf("expected valid arguments to pass, got: %v", err)
	}

	// 2. Missing required field
	missingRequired := map[string]model.Expr{
		"priority": model.Literal(priorityVal),
	}
	if err := ValidateActionArguments(normDesc, missingRequired, env); err == nil {
		t.Fatalf("expected missing required field to fail")
	}

	// 3. Unknown field rejection
	unknownField := map[string]model.Expr{
		"title":         model.Literal(titleVal),
		"unknown_field": model.Literal(intVal),
	}
	if err := ValidateActionArguments(normDesc, unknownField, env); err == nil {
		t.Fatalf("expected unknown field to fail")
	}

	// 4. Type mismatch
	typeMismatch := map[string]model.Expr{
		"title": model.Literal(intVal), // expects string, got int
	}
	if err := ValidateActionArguments(normDesc, typeMismatch, env); err == nil {
		t.Fatalf("expected type mismatch to fail")
	}

	// 5. Enum mismatch
	enumMismatch := map[string]model.Expr{
		"title":    model.Literal(titleVal),
		"priority": model.Literal(badPriorityVal),
	}
	if err := ValidateActionArguments(normDesc, enumMismatch, env); err == nil {
		t.Fatalf("expected enum mismatch to fail")
	}
}
