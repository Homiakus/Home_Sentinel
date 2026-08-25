package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Expr struct {
	Op    string `json:"op,omitempty"`
	Ref   string `json:"ref,omitempty"`
	Value *Value `json:"value,omitempty"`
	Args  []Expr `json:"args,omitempty"`
}

type TypeEnv map[string]TypeRef

func (e Expr) IsZero() bool {
	return e.Op == "" && e.Ref == "" && e.Value == nil && len(e.Args) == 0
}

func (e Expr) Validate() error {
	if e.IsZero() {
		return nil
	}
	op := strings.ToLower(strings.TrimSpace(e.Op))
	switch op {
	case "literal":
		if e.Value == nil || e.Ref != "" || len(e.Args) != 0 {
			return fmt.Errorf("scenario: literal expression requires only value")
		}
		_, err := e.Value.canonical()
		return err
	case "ref":
		if strings.TrimSpace(e.Ref) == "" || e.Value != nil || len(e.Args) != 0 {
			return fmt.Errorf("scenario: ref expression requires only ref")
		}
		return validateReference(e.Ref)
	case "not", "exists", "missing", "changed":
		if len(e.Args) != 1 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: %s expression requires one argument", op)
		}
	case "and", "or":
		if len(e.Args) < 2 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: %s expression requires at least two arguments", op)
		}
	case "eq", "ne", "gt", "gte", "lt", "lte", "in", "add", "sub", "mul", "div":
		if len(e.Args) != 2 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: %s expression requires two arguments", op)
		}
	case "between":
		if len(e.Args) != 3 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: between expression requires value, lower and upper arguments")
		}
	default:
		return fmt.Errorf("scenario: unknown expression operator %q", e.Op)
	}
	for i := range e.Args {
		if e.Args[i].IsZero() {
			return fmt.Errorf("scenario: %s expression argument %d is empty", op, i)
		}
		if err := e.Args[i].Validate(); err != nil {
			return fmt.Errorf("scenario: %s argument %d: %w", op, i, err)
		}
	}
	return nil
}

func CheckExpr(e Expr, env TypeEnv) (TypeRef, error) {
	if err := e.Validate(); err != nil {
		return TypeRef{}, err
	}
	op := strings.ToLower(strings.TrimSpace(e.Op))
	switch op {
	case "literal":
		value, err := e.Value.canonical()
		if err != nil {
			return TypeRef{}, err
		}
		return value.Type, nil
	case "ref":
		typ, ok := env[strings.TrimSpace(e.Ref)]
		if !ok {
			return TypeRef{}, fmt.Errorf("scenario: unknown expression reference %q", e.Ref)
		}
		typ, err := typ.Normalize()
		if err != nil {
			return TypeRef{}, fmt.Errorf("scenario: reference %q: %w", e.Ref, err)
		}
		return typ, nil
	case "exists", "missing":
		if _, err := CheckExpr(e.Args[0], env); err != nil {
			return TypeRef{}, err
		}
		return TypeRef{Kind: TypeBool}, nil
	case "changed":
		if strings.ToLower(strings.TrimSpace(e.Args[0].Op)) != "ref" {
			return TypeRef{}, fmt.Errorf("scenario: changed requires a reference")
		}
		if _, err := CheckExpr(e.Args[0], env); err != nil {
			return TypeRef{}, err
		}
		return TypeRef{Kind: TypeBool}, nil
	case "not":
		typ, err := CheckExpr(e.Args[0], env)
		if err != nil {
			return TypeRef{}, err
		}
		if typ.Kind != TypeBool {
			return TypeRef{}, fmt.Errorf("scenario: not requires bool, got %q", typ.Kind)
		}
		return TypeRef{Kind: TypeBool}, nil
	case "and", "or":
		for i := range e.Args {
			typ, err := CheckExpr(e.Args[i], env)
			if err != nil {
				return TypeRef{}, err
			}
			if typ.Kind != TypeBool {
				return TypeRef{}, fmt.Errorf("scenario: %s argument %d requires bool, got %q", op, i, typ.Kind)
			}
		}
		return TypeRef{Kind: TypeBool}, nil
	case "eq", "ne", "gt", "gte", "lt", "lte":
		left, right, err := checkBinaryTypes(e, env)
		if err != nil {
			return TypeRef{}, err
		}
		if !left.Compatible(right) {
			return TypeRef{}, fmt.Errorf("scenario: %s cannot compare %q with %q", op, left.Kind, right.Kind)
		}
		if op != "eq" && op != "ne" && !left.Ordered() {
			return TypeRef{}, fmt.Errorf("scenario: %s does not support ordering for %q", op, left.Kind)
		}
		return TypeRef{Kind: TypeBool}, nil
	case "in":
		item, list, err := checkBinaryTypes(e, env)
		if err != nil {
			return TypeRef{}, err
		}
		if list.Kind != TypeList || list.Element == nil || !item.Compatible(*list.Element) {
			return TypeRef{}, fmt.Errorf("scenario: in requires list of %q", item.Kind)
		}
		return TypeRef{Kind: TypeBool}, nil
	case "between":
		valueType, err := CheckExpr(e.Args[0], env)
		if err != nil {
			return TypeRef{}, err
		}
		lower, err := CheckExpr(e.Args[1], env)
		if err != nil {
			return TypeRef{}, err
		}
		upper, err := CheckExpr(e.Args[2], env)
		if err != nil {
			return TypeRef{}, err
		}
		if !valueType.Ordered() || !valueType.Compatible(lower) || !valueType.Compatible(upper) {
			return TypeRef{}, fmt.Errorf("scenario: between requires three compatible ordered values")
		}
		return TypeRef{Kind: TypeBool}, nil
	case "add", "sub", "mul", "div":
		left, right, err := checkBinaryTypes(e, env)
		if err != nil {
			return TypeRef{}, err
		}
		return arithmeticResult(op, left, right)
	default:
		return TypeRef{}, fmt.Errorf("scenario: unsupported expression operator %q", op)
	}
}

func checkBinaryTypes(e Expr, env TypeEnv) (TypeRef, TypeRef, error) {
	left, err := CheckExpr(e.Args[0], env)
	if err != nil {
		return TypeRef{}, TypeRef{}, err
	}
	right, err := CheckExpr(e.Args[1], env)
	if err != nil {
		return TypeRef{}, TypeRef{}, err
	}
	return left, right, nil
}

func arithmeticResult(op string, left, right TypeRef) (TypeRef, error) {
	if op == "add" && left.Kind == TypeString && right.Kind == TypeString {
		return TypeRef{Kind: TypeString}, nil
	}
	if !left.Numeric() || !right.Numeric() {
		return TypeRef{}, fmt.Errorf("scenario: %s requires numeric operands", op)
	}
	if left.Kind == TypeInt && right.Kind == TypeInt {
		if op == "div" {
			return TypeRef{Kind: TypeFloat}, nil
		}
		return TypeRef{Kind: TypeInt}, nil
	}
	if (left.Kind == TypeInt || left.Kind == TypeFloat) && (right.Kind == TypeInt || right.Kind == TypeFloat) {
		return TypeRef{Kind: TypeFloat}, nil
	}
	if !left.Compatible(right) {
		return TypeRef{}, fmt.Errorf("scenario: %s requires compatible quantity dimensions, got %q and %q", op, left.Kind, right.Kind)
	}
	if op == "mul" || op == "div" {
		return TypeRef{}, fmt.Errorf("scenario: %s on unit-aware quantities is not allowed without an explicit dimensional rule", op)
	}
	return left, nil
}

func validateReference(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || len(ref) > 256 {
		return fmt.Errorf("scenario: invalid expression reference")
	}
	for _, part := range strings.Split(ref, ".") {
		if err := validateToken("reference segment", part); err != nil {
			return err
		}
	}
	return nil
}

func normalizeExpr(e Expr) (Expr, error) {
	if e.IsZero() {
		return Expr{}, nil
	}
	e.Op = strings.ToLower(strings.TrimSpace(e.Op))
	e.Ref = strings.TrimSpace(e.Ref)
	if e.Value != nil {
		canonical, err := e.Value.canonical()
		if err != nil {
			return Expr{}, err
		}
		e.Value = &canonical
	}
	for i := range e.Args {
		normalized, err := normalizeExpr(e.Args[i])
		if err != nil {
			return Expr{}, err
		}
		e.Args[i] = normalized
	}
	if e.Op == "and" || e.Op == "or" {
		sort.SliceStable(e.Args, func(i, j int) bool {
			left, _ := json.Marshal(e.Args[i])
			right, _ := json.Marshal(e.Args[j])
			return bytes.Compare(left, right) < 0
		})
	}
	if err := e.Validate(); err != nil {
		return Expr{}, err
	}
	return e, nil
}
