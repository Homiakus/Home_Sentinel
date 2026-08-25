package storage

import "errors"

type Class string

const (
	ClassSystem     Class = "system"
	ClassRecordings Class = "recordings"
	ClassBackup     Class = "backup"
	ClassCache      Class = "cache"
)

type Thresholds struct {
	WarningFreePercent        float64 `json:"warning_free_percent"`
	CriticalFreePercent       float64 `json:"critical_free_percent"`
	RecoveryHysteresisPercent float64 `json:"recovery_hysteresis_percent"`
}
type Policy struct {
	Class            Class      `json:"class"`
	MountPoint       string     `json:"mount_point"`
	Thresholds       Thresholds `json:"thresholds"`
	MinimumFreeBytes uint64     `json:"minimum_free_bytes,omitempty"`
}

func (p Policy) Validate() error {
	switch p.Class {
	case ClassSystem, ClassRecordings, ClassBackup, ClassCache:
	default:
		return errors.New("invalid storage class")
	}
	if p.MountPoint == "" {
		return errors.New("storage mount point required")
	}
	if p.Thresholds.WarningFreePercent <= p.Thresholds.CriticalFreePercent || p.Thresholds.CriticalFreePercent < 0 || p.Thresholds.WarningFreePercent > 100 {
		return errors.New("storage free-space thresholds invalid")
	}
	if p.Thresholds.RecoveryHysteresisPercent < 0 || p.Thresholds.RecoveryHysteresisPercent > 20 {
		return errors.New("storage hysteresis invalid")
	}
	return nil
}
func DefaultPolicy(class Class, mount string) Policy {
	return Policy{Class: class, MountPoint: mount, Thresholds: Thresholds{WarningFreePercent: 15, CriticalFreePercent: 7, RecoveryHysteresisPercent: 3}}
}
