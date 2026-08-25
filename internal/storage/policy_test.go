package storage

import "testing"

func TestEstimateRetention(t *testing.T) {
	got := EstimateRetention(4_000_000, 30, 2_000_000_000_000)
	if got.BytesPerDay < 43_000_000_000 || got.BytesPerDay > 44_000_000_000 {
		t.Fatalf("unexpected daily bytes: %d", got.BytesPerDay)
	}
	if got.EstimatedDaysAtFreeSpace < 45 || got.EstimatedDaysAtFreeSpace > 47 {
		t.Fatalf("unexpected retention days: %v", got.EstimatedDaysAtFreeSpace)
	}
}

func TestGuardHysteresis(t *testing.T) {
	g := &Guard{Policy: Policy{Class: ClassSystem, MountPoint: "/", Thresholds: Thresholds{WarningFreePercent: 15, CriticalFreePercent: 8, RecoveryHysteresisPercent: 3}}}
	if got, err := g.Evaluate(Sample{Total: 1000, Free: 140}); err != nil || got != Warning {
		t.Fatalf("want warning, got %s err=%v", got, err)
	}
	if got, err := g.Evaluate(Sample{Total: 1000, Free: 160}); err != nil || got != Warning {
		t.Fatalf("hysteresis should keep warning, got %s err=%v", got, err)
	}
	if got, err := g.Evaluate(Sample{Total: 1000, Free: 190}); err != nil || got != Normal {
		t.Fatalf("want recovery to normal, got %s err=%v", got, err)
	}
	if got, err := g.Evaluate(Sample{Total: 1000, Free: 70}); err != nil || got != Critical {
		t.Fatalf("want critical, got %s err=%v", got, err)
	}
}
