package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type ConfigRevision struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Actor     string    `json:"actor"`
	Reason    string    `json:"reason"`
	Checksum  string    `json:"checksum"`
	Document  []byte    `json:"document"`
}

type RevisionStore struct{ db *sql.DB }

func NewRevisionStore(db *sql.DB) *RevisionStore { return &RevisionStore{db: db} }

func (s *RevisionStore) Create(ctx context.Context, actor, reason string, document []byte) (ConfigRevision, error) {
	if actor == "" {
		actor = "system"
	}
	if len(document) == 0 {
		return ConfigRevision{}, errors.New("revision document required")
	}
	sum := sha256.Sum256(document)
	checksum := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO config_revisions(created_at,actor,reason,checksum,document) VALUES(?,?,?,?,?)`, now.Format(time.RFC3339Nano), actor, reason, checksum, document)
	if err != nil {
		return ConfigRevision{}, fmt.Errorf("create revision: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ConfigRevision{}, err
	}
	return ConfigRevision{ID: id, CreatedAt: now, Actor: actor, Reason: reason, Checksum: checksum, Document: append([]byte(nil), document...)}, nil
}
func (s *RevisionStore) Get(ctx context.Context, id int64) (ConfigRevision, error) {
	var r ConfigRevision
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT id,created_at,actor,reason,checksum,document FROM config_revisions WHERE id=?`, id).Scan(&r.ID, &raw, &r.Actor, &r.Reason, &r.Checksum, &r.Document)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfigRevision{}, ErrNotFound
	}
	if err != nil {
		return ConfigRevision{}, err
	}
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, raw)
	return r, err
}
func (s *RevisionStore) Latest(ctx context.Context) (ConfigRevision, error) {
	var r ConfigRevision
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT id,created_at,actor,reason,checksum,document FROM config_revisions ORDER BY id DESC LIMIT 1`).Scan(&r.ID, &raw, &r.Actor, &r.Reason, &r.Checksum, &r.Document)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfigRevision{}, ErrNotFound
	}
	if err != nil {
		return ConfigRevision{}, err
	}
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, raw)
	return r, err
}
func (s *RevisionStore) List(ctx context.Context, limit int) ([]ConfigRevision, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,actor,reason,checksum,document FROM config_revisions ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConfigRevision
	for rows.Next() {
		var r ConfigRevision
		var raw string
		if err := rows.Scan(&r.ID, &raw, &r.Actor, &r.Reason, &r.Checksum, &r.Document); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, raw)
		out = append(out, r)
	}
	return out, rows.Err()
}
