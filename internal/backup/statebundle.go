package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const BundleSchemaV1 = 1

type RawResource struct {
	Kind, ID, CreatedAt, UpdatedAt string
	Revision                       int64
	Payload                        json.RawMessage
}
type RawRevision struct {
	ID                                 int64
	CreatedAt, Actor, Reason, Checksum string
	Document                           []byte
}
type StateBundle struct {
	Schema     int           `json:"schema"`
	ExportedAt time.Time     `json:"exported_at"`
	Resources  []RawResource `json:"resources"`
	Revisions  []RawRevision `json:"revisions"`
}

func ExportState(ctx context.Context, db *sql.DB, w io.Writer) error {
	if db == nil || w == nil {
		return errors.New("database and writer required")
	}
	b := StateBundle{Schema: BundleSchemaV1, ExportedAt: time.Now().UTC()}
	rows, err := db.QueryContext(ctx, `SELECT kind,id,created_at,updated_at,revision,payload FROM resources ORDER BY kind,id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var r RawResource
		var rawPayload any
		if err := rows.Scan(&r.Kind, &r.ID, &r.CreatedAt, &r.UpdatedAt, &r.Revision, &rawPayload); err != nil {
			rows.Close()
			return err
		}
		payload, err := normalizeJSONColumn(rawPayload)
		if err != nil {
			rows.Close()
			return fmt.Errorf("resource %s/%s payload: %w", r.Kind, r.ID, err)
		}
		r.Payload = payload
		b.Resources = append(b.Resources, r)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	rr, err := db.QueryContext(ctx, `SELECT id,created_at,actor,reason,checksum,document FROM config_revisions ORDER BY id`)
	if err != nil {
		return err
	}
	defer rr.Close()
	for rr.Next() {
		var r RawRevision
		if err := rr.Scan(&r.ID, &r.CreatedAt, &r.Actor, &r.Reason, &r.Checksum, &r.Document); err != nil {
			return err
		}
		b.Revisions = append(b.Revisions, r)
	}
	if err := rr.Err(); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}

func normalizeJSONColumn(value any) (json.RawMessage, error) {
	var raw []byte
	switch x := value.(type) {
	case []byte:
		raw = x
	case string:
		raw = []byte(x)
	case nil:
		return nil, errors.New("null JSON column")
	default:
		return nil, fmt.Errorf("unsupported JSON column type %T", value)
	}
	if !json.Valid(raw) {
		return nil, errors.New("invalid JSON column")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func ImportState(ctx context.Context, db *sql.DB, r io.Reader) error {
	if db == nil || r == nil {
		return errors.New("database and reader required")
	}
	var b StateBundle
	dec := json.NewDecoder(io.LimitReader(r, 64<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return fmt.Errorf("decode state bundle: %w", err)
	}
	if b.Schema != BundleSchemaV1 {
		return fmt.Errorf("unsupported state bundle schema %d", b.Schema)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM resources`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM config_revisions`); err != nil {
		return err
	}
	for _, x := range b.Resources {
		if x.Kind == "" || x.ID == "" || !json.Valid(x.Payload) {
			return errors.New("invalid resource in bundle")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO resources(kind,id,created_at,updated_at,revision,payload) VALUES(?,?,?,?,?,?)`, x.Kind, x.ID, x.CreatedAt, x.UpdatedAt, x.Revision, []byte(x.Payload)); err != nil {
			return err
		}
	}
	for _, x := range b.Revisions {
		if _, err = tx.ExecContext(ctx, `INSERT INTO config_revisions(id,created_at,actor,reason,checksum,document) VALUES(?,?,?,?,?,?)`, x.ID, x.CreatedAt, x.Actor, x.Reason, x.Checksum, x.Document); err != nil {
			return err
		}
	}
	return tx.Commit()
}
