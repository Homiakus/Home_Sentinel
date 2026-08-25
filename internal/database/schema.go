package database

import (
	"context"
	"database/sql"
	"errors"
)

func SchemaVersion(ctx context.Context, db *sql.DB) (int64, error) {
	if db == nil {
		return 0, errors.New("nil database")
	}
	var v sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return v.Int64, nil
}
