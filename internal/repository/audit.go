package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type AuditEntry struct {
	Seq           int64           `json:"seq"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Actor         string          `json:"actor"`
	Source        string          `json:"source"`
	Action        string          `json:"action"`
	Target        string          `json:"target"`
	Result        string          `json:"result"`
	RequestID     string          `json:"request_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Before        json.RawMessage `json:"before,omitempty"`
	After         json.RawMessage `json:"after,omitempty"`
	Details       json.RawMessage `json:"details,omitempty"`
}

type AuditStore struct{ db *sql.DB }

func NewAuditStore(db *sql.DB) *AuditStore { return &AuditStore{db: db} }
func (s *AuditStore) Append(ctx context.Context, e AuditEntry) (AuditEntry, error) {
	if e.Actor == "" || e.Action == "" || e.Target == "" || e.Result == "" {
		return AuditEntry{}, errors.New("audit actor/action/target/result required")
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO audit_log(occurred_at,actor,source,action,target,result,request_id,correlation_id,before_json,after_json,details_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, e.OccurredAt.Format(time.RFC3339Nano), e.Actor, e.Source, e.Action, e.Target, e.Result, e.RequestID, e.CorrelationID, nullBytes(e.Before), nullBytes(e.After), nullBytes(e.Details))
	if err != nil {
		return AuditEntry{}, err
	}
	e.Seq, _ = res.LastInsertId()
	return e, nil
}
func nullBytes(v []byte) any {
	if len(v) == 0 {
		return nil
	}
	return v
}
