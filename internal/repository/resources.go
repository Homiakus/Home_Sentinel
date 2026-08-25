package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("resource not found")

type Kind string

const (
	KindDevice        Kind = "device"
	KindCamera        Kind = "camera"
	KindEvent         Kind = "event"
	KindIncident      Kind = "incident"
	KindUser          Kind = "user"
	KindPolicy        Kind = "policy"
	KindBackupJob     Kind = "backup_job"
	KindIntegration   Kind = "integration"
	KindIntercom      Kind = "intercom"
	KindIntercomState Kind = "intercom_state"
	KindSetup         Kind = "setup"
	KindUpdate        Kind = "update"
)

type Resource[T any] struct {
	Kind      Kind      `json:"kind"`
	ID        string    `json:"id"`
	Revision  int64     `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Value     T         `json:"value"`
}

type Store[T any] struct {
	db   *sql.DB
	kind Kind
}

func NewStore[T any](db *sql.DB, kind Kind) *Store[T] {
	return &Store[T]{db: db, kind: kind}
}

func (s *Store[T]) Put(ctx context.Context, id string, value T) (Resource[T], error) {
	if s == nil || s.db == nil {
		return Resource[T]{}, errors.New("repository database unavailable")
	}
	if id == "" || s.kind == "" {
		return Resource[T]{}, errors.New("resource kind and id required")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return Resource[T]{}, fmt.Errorf("marshal %s %s: %w", s.kind, id, err)
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Resource[T]{}, err
	}
	defer tx.Rollback()
	var createdRaw, updatedRaw string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT created_at, updated_at, revision FROM resources WHERE kind=? AND id=?`, string(s.kind), id).Scan(&createdRaw, &updatedRaw, &revision)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		revision = 1
		createdRaw = now.Format(time.RFC3339Nano)
		updatedRaw = createdRaw
		if _, err = tx.ExecContext(ctx, `INSERT INTO resources(kind,id,created_at,updated_at,revision,payload) VALUES(?,?,?,?,?,?)`, string(s.kind), id, createdRaw, updatedRaw, revision, payload); err != nil {
			return Resource[T]{}, fmt.Errorf("insert %s %s: %w", s.kind, id, err)
		}
	case err != nil:
		return Resource[T]{}, fmt.Errorf("lookup %s %s: %w", s.kind, id, err)
	default:
		revision++
		updatedRaw = now.Format(time.RFC3339Nano)
		if _, err = tx.ExecContext(ctx, `UPDATE resources SET updated_at=?, revision=?, payload=? WHERE kind=? AND id=?`, updatedRaw, revision, payload, string(s.kind), id); err != nil {
			return Resource[T]{}, fmt.Errorf("update %s %s: %w", s.kind, id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Resource[T]{}, err
	}
	created, _ := time.Parse(time.RFC3339Nano, createdRaw)
	updated, _ := time.Parse(time.RFC3339Nano, updatedRaw)
	return Resource[T]{Kind: s.kind, ID: id, Revision: revision, CreatedAt: created, UpdatedAt: updated, Value: value}, nil
}

func (s *Store[T]) Get(ctx context.Context, id string) (Resource[T], error) {
	var payload []byte
	var createdRaw, updatedRaw string
	var revision int64
	err := s.db.QueryRowContext(ctx, `SELECT created_at,updated_at,revision,payload FROM resources WHERE kind=? AND id=?`, string(s.kind), id).Scan(&createdRaw, &updatedRaw, &revision, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return Resource[T]{}, ErrNotFound
	}
	if err != nil {
		return Resource[T]{}, err
	}
	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return Resource[T]{}, fmt.Errorf("decode %s %s: %w", s.kind, id, err)
	}
	created, err := time.Parse(time.RFC3339Nano, createdRaw)
	if err != nil {
		return Resource[T]{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedRaw)
	if err != nil {
		return Resource[T]{}, err
	}
	return Resource[T]{Kind: s.kind, ID: id, Revision: revision, CreatedAt: created, UpdatedAt: updated, Value: value}, nil
}

func (s *Store[T]) List(ctx context.Context, limit int) ([]Resource[T], error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,updated_at,revision,payload FROM resources WHERE kind=? ORDER BY updated_at DESC,id LIMIT ?`, string(s.kind), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Resource[T], 0)
	for rows.Next() {
		var id, cr, ur string
		var rev int64
		var payload []byte
		if err := rows.Scan(&id, &cr, &ur, &rev, &payload); err != nil {
			return nil, err
		}
		var value T
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, err
		}
		created, _ := time.Parse(time.RFC3339Nano, cr)
		updated, _ := time.Parse(time.RFC3339Nano, ur)
		out = append(out, Resource[T]{Kind: s.kind, ID: id, Revision: rev, CreatedAt: created, UpdatedAt: updated, Value: value})
	}
	return out, rows.Err()
}

func (s *Store[T]) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM resources WHERE kind=? AND id=?`, string(s.kind), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
