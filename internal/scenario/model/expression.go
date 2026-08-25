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
	case "not", "exists":
		if len(e.Args) != 1 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: %s expression requires one argument", op)
		}
	case "and", "or":
		if len(e.Args) < 2 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: %s expression requires at least two arguments", op)
		}
	case "eq", "ne", "gt", "gte", "lt", "lte", "in":
		if len(e.Args) != 2 || e.Ref != "" || e.Value != nil {
			return fmt.Errorf("scenario: %s expression requires two arguments", op)
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
