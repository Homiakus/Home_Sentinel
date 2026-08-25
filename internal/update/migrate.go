package update

import (
	"errors"
	"fmt"
)

type MigrationDecision struct {
	Allowed                    bool   `json:"allowed"`
	BackupRequired             bool   `json:"backup_required"`
	RestoreRequiredForRollback bool   `json:"restore_required_for_rollback"`
	Reason                     string `json:"reason,omitempty"`
}

func DecideMigration(current, target, minReadable int64, irreversible bool) MigrationDecision {
	d := MigrationDecision{BackupRequired: true}
	switch {
	case current <= 0 || target <= 0 || minReadable <= 0:
		d.Reason = "invalid schema version"
	case current > target:
		d.Reason = "downgrade migrations are not executed in place"
		d.RestoreRequiredForRollback = true
	case current < minReadable:
		d.Reason = fmt.Sprintf("schema %d is outside supported migration window >=%d", current, minReadable)
	case current == target:
		d.Allowed = true
	case irreversible:
		d.Allowed = true
		d.RestoreRequiredForRollback = true
		d.Reason = "migration is irreversible; rollback requires checkpoint restore"
	default:
		d.Allowed = true
	}
	return d
}
func RequireMigration(d MigrationDecision) error {
	if !d.Allowed {
		return errors.New(d.Reason)
	}
	return nil
}
