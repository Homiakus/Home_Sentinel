package simulator

import (
	"fmt"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type EvalEnv map[string]model.Value

// Evaluate evaluates an Expr against an evaluation environment and returns the resulting model.Value.
func Evaluate(e model.Expr, env EvalEnv) (model.Value, error) {
	if e.IsZero() {
		return model.Value{}, fmt.Errorf("eval: cannot evaluate empty expression")
	}

	op := strings.ToLower(strings.TrimSpace(e.Op))
	switch op {
	case "literal":
		if e.Value == nil {
			return model.Value{}, fmt.Errorf("eval: literal expression has nil value")
		}
		return *e.Value, nil

	case "ref":
		refKey := strings.TrimSpace(e.Ref)
		val, ok := env[refKey]
		if !ok {
			return model.Value{}, fmt.Errorf("eval: reference %q not found in runtime environment", refKey)
		}
		return val, nil

	case "not":
		arg, err := Evaluate(e.Args[0], env)
		if err != nil {
			return model.Value{}, err
		}
		var b bool
		if err := arg.Unmarshal(&b); err != nil {
			return model.Value{}, fmt.Errorf("eval: not requires bool value: %w", err)
		}
		return model.ValueOf(!b)

	case "exists":
		ref := e.Args[0].Ref
		_, exists := env[ref]
		return model.ValueOf(exists)

	case "missing":
		ref := e.Args[0].Ref
		_, exists := env[ref]
		return model.ValueOf(!exists)

	case "and":
		for i, arg := range e.Args {
			val, err := Evaluate(arg, env)
			if err != nil {
				return model.Value{}, fmt.Errorf("eval: and argument %d: %w", i, err)
			}
			var b bool
			if err := val.Unmarshal(&b); err != nil {
				return model.Value{}, fmt.Errorf("eval: and argument %d not a bool: %w", i, err)
			}
			if !b {
				return model.ValueOf(false)
			}
		}
		return model.ValueOf(true)

	case "or":
		for i, arg := range e.Args {
			val, err := Evaluate(arg, env)
			if err != nil {
				return model.Value{}, fmt.Errorf("eval: or argument %d: %w", i, err)
			}
			var b bool
			if err := val.Unmarshal(&b); err != nil {
				return model.Value{}, fmt.Errorf("eval: or argument %d not a bool: %w", i, err)
			}
			if b {
				return model.ValueOf(true)
			}
		}
		return model.ValueOf(false)

	case "eq", "ne":
		left, err := Evaluate(e.Args[0], env)
		if err != nil {
			return model.Value{}, err
		}
		right, err := Evaluate(e.Args[1], env)
		if err != nil {
			return model.Value{}, err
		}
		equal := string(left.Data) == string(right.Data)
		if op == "eq" {
			return model.ValueOf(equal)
		}
		return model.ValueOf(!equal)

	case "gt", "gte", "lt", "lte":
		left, err := Evaluate(e.Args[0], env)
		if err != nil {
			return model.Value{}, err
		}
		right, err := Evaluate(e.Args[1], env)
		if err != nil {
			return model.Value{}, err
		}

		var leftNum, rightNum float64
		if err := left.Unmarshal(&leftNum); err != nil {
			return model.Value{}, fmt.Errorf("eval: ordering requires numeric left operand")
		}
		if err := right.Unmarshal(&rightNum); err != nil {
			return model.Value{}, fmt.Errorf("eval: ordering requires numeric right operand")
		}

		switch op {
		case "gt":
			return model.ValueOf(leftNum > rightNum)
		case "gte":
			return model.ValueOf(leftNum >= rightNum)
		case "lt":
			return model.ValueOf(leftNum < rightNum)
		case "lte":
			return model.ValueOf(leftNum <= rightNum)
		}

	case "between":
		val, err := Evaluate(e.Args[0], env)
		if err != nil {
			return model.Value{}, err
		}
		lower, err := Evaluate(e.Args[1], env)
		if err != nil {
			return model.Value{}, err
		}
		upper, err := Evaluate(e.Args[2], env)
		if err != nil {
			return model.Value{}, err
		}

		var v, l, u float64
		if err := val.Unmarshal(&v); err != nil {
			return model.Value{}, fmt.Errorf("eval: between requires numeric target")
		}
		if err := lower.Unmarshal(&l); err != nil {
			return model.Value{}, fmt.Errorf("eval: between requires numeric lower bound")
		}
		if err := upper.Unmarshal(&u); err != nil {
			return model.Value{}, fmt.Errorf("eval: between requires numeric upper bound")
		}

		return model.ValueOf(v >= l && v <= u)

	case "add", "sub", "mul", "div":
		left, err := Evaluate(e.Args[0], env)
		if err != nil {
			return model.Value{}, err
		}
		right, err := Evaluate(e.Args[1], env)
		if err != nil {
			return model.Value{}, err
		}

		// String concatenation for add
		if op == "add" && left.Type.Kind == model.TypeString && right.Type.Kind == model.TypeString {
			var s1, s2 string
			_ = left.Unmarshal(&s1)
			_ = right.Unmarshal(&s2)
			return model.ValueOf(s1 + s2)
		}

		var n1, n2 float64
		if err := left.Unmarshal(&n1); err != nil {
			return model.Value{}, fmt.Errorf("eval: arithmetic requires numeric left operand")
		}
		if err := right.Unmarshal(&n2); err != nil {
			return model.Value{}, fmt.Errorf("eval: arithmetic requires numeric right operand")
		}

		switch op {
		case "add":
			return model.ValueOf(n1 + n2)
		case "sub":
			return model.ValueOf(n1 - n2)
		case "mul":
			return model.ValueOf(n1 * n2)
		case "div":
			if n2 == 0 {
				return model.Value{}, fmt.Errorf("eval: division by zero")
			}
			return model.ValueOf(n1 / n2)
		}
	}

	return model.Value{}, fmt.Errorf("eval: unsupported operator %q", op)
}

// EvaluateBool evaluates an expression to a boolean.
func EvaluateBool(e model.Expr, env EvalEnv) (bool, error) {
	val, err := Evaluate(e, env)
	if err != nil {
		return false, err
	}
	var b bool
	if err := val.Unmarshal(&b); err != nil {
		return false, fmt.Errorf("eval: expected boolean result: %w", err)
	}
	return b, nil
}
