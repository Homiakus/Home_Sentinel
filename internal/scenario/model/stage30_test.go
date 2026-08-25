package model

import (
	"math"
	"testing"
	"time"
)

func TestQuantityConversion(t *testing.T) {
	value, err := NewQuantity(TypeTemperature, 68, "fahrenheit")
	if err != nil {
		t.Fatal(err)
	}
	number, err := numberValue(value)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(number-20) > 1e-9 {
		t.Fatalf("got %f", number)
	}
	distance, err := NewQuantity(TypeDistance, 250, "cm")
	if err != nil {
		t.Fatal(err)
	}
	number, _ = numberValue(distance)
	if math.Abs(number-2.5) > 1e-9 {
		t.Fatalf("distance got %f", number)
	}
}

func TestTypeMismatch(t *testing.T) {
	temp, _ := NewQuantity(TypeTemperature, 20, "celsius")
	text, _ := ValueOf("hello")
	expr := Expr{Op: "gt", Args: []Expr{{Op: "literal", Value: &temp}, {Op: "literal", Value: &text}}}
	if _, err := CheckExpr(expr, nil); err == nil {
		t.Fatal("accepted mismatch")
	}
}

func TestConfidenceRequiresTypedLiteral(t *testing.T) {
	threshold, _ := NewQuantity(TypeConfidence, 60, "%")
	expr := Expr{Op: "gte", Args: []Expr{{Op: "ref", Ref: "person.confidence"}, {Op: "literal", Value: &threshold}}}
	typ, err := CheckExpr(expr, TypeEnv{"person.confidence": {Kind: TypeConfidence, Unit: "ratio"}})
	if err != nil {
		t.Fatal(err)
	}
	if typ.Kind != TypeBool {
		t.Fatal(typ.Kind)
	}
}

func TestMidnightWindow(t *testing.T) {
	start, _ := ParseTimeOfDay("22:00")
	end, _ := ParseTimeOfDay("07:00")
	at, _ := ParseTimeOfDay("01:30")
	noon, _ := ParseTimeOfDay("12:00")
	if !TimeWindowContains(start, end, at) {
		t.Fatal("01:30 should be inside")
	}
	if TimeWindowContains(start, end, noon) {
		t.Fatal("noon should be outside")
	}
}

func TestDSTResolutionAmsterdam(t *testing.T) {
	spring := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	wall, _ := ParseTimeOfDay("02:30")
	if _, ok, err := ResolveWallClock(spring, wall, "Europe/Amsterdam", DSTSkipInvalid); err != nil || ok {
		t.Fatalf("nonexistent: ok=%v err=%v", ok, err)
	}
	fall := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	first, ok, err := ResolveWallClock(fall, wall, "Europe/Amsterdam", DSTWallClockFirst)
	if err != nil || !ok {
		t.Fatal(err)
	}
	last, ok, err := ResolveWallClock(fall, wall, "Europe/Amsterdam", DSTWallClockLast)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if last.Sub(first) != time.Hour {
		t.Fatalf("expected 1h ambiguity, got %v", last.Sub(first))
	}
}

func TestRepeatWithinValidation(t *testing.T) {
	if err := (TemporalSpec{Kind: TemporalRepeatWithin, Count: 3, Duration: 30 * time.Second}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (TemporalSpec{Kind: TemporalRepeatWithin, Count: 1, Duration: 30 * time.Second}).Validate(); err == nil {
		t.Fatal("accepted count 1")
	}
}

func TestIntAndLegacyScalarValue(t *testing.T) {
	value, err := ValueOf(42)
	if err != nil {
		t.Fatal(err)
	}
	if value.Type.Kind != TypeInt {
		t.Fatalf("got %s", value.Type.Kind)
	}
	var legacy Value
	if err := legacy.UnmarshalJSON([]byte(`42`)); err != nil {
		t.Fatal(err)
	}
	if legacy.Type.Kind != TypeInt {
		t.Fatalf("legacy got %s", legacy.Type.Kind)
	}
}
