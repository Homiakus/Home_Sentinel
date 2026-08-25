package events

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

type OutboxMessage struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	AvailableAt time.Time `json:"available_at"`
	Attempts    int       `json:"attempts"`
	Destination string    `json:"destination"`
	Payload     []byte    `json:"payload"`
}
type Outbox struct{ DB *sql.DB }

func (o Outbox) EnqueueTx(ctx context.Context, tx *sql.Tx, destination string, payload []byte, available time.Time) (OutboxMessage, error) {
	if tx == nil {
		return OutboxMessage{}, errors.New("outbox transaction required")
	}
	if destination == "" || len(payload) == 0 {
		return OutboxMessage{}, errors.New("destination and payload required")
	}
	id, err := domain.NewID("out")
	if err != nil {
		return OutboxMessage{}, err
	}
	now := time.Now().UTC()
	if available.IsZero() {
		available = now
	}
	m := OutboxMessage{ID: id.String(), CreatedAt: now, AvailableAt: available, Destination: destination, Payload: append([]byte(nil), payload...)}
	_, err = tx.ExecContext(ctx, `INSERT INTO event_outbox(id,created_at,available_at,destination,payload) VALUES(?,?,?,?,?)`, m.ID, now.Format(time.RFC3339Nano), available.UTC().Format(time.RFC3339Nano), destination, payload)
	return m, err
}
func (o Outbox) Enqueue(ctx context.Context, destination string, payload []byte) (OutboxMessage, error) {
	if o.DB == nil {
		return OutboxMessage{}, errors.New("outbox database unavailable")
	}
	tx, err := o.DB.BeginTx(ctx, nil)
	if err != nil {
		return OutboxMessage{}, err
	}
	defer tx.Rollback()
	m, err := o.EnqueueTx(ctx, tx, destination, payload, time.Time{})
	if err != nil {
		return OutboxMessage{}, err
	}
	if err := tx.Commit(); err != nil {
		return OutboxMessage{}, err
	}
	return m, nil
}
func (o Outbox) Claim(ctx context.Context, limit int, lease time.Duration) ([]OutboxMessage, error) {
	if o.DB == nil {
		return nil, errors.New("outbox database unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	tx, err := o.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,created_at,available_at,attempts,destination,payload FROM event_outbox WHERE delivered_at IS NULL AND available_at<=? AND (claimed_until IS NULL OR claimed_until<?) ORDER BY created_at,id LIMIT ?`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	var out []OutboxMessage
	for rows.Next() {
		var m OutboxMessage
		var cr, av string
		if err := rows.Scan(&m.ID, &cr, &av, &m.Attempts, &m.Destination, &m.Payload); err != nil {
			rows.Close()
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, cr)
		m.AvailableAt, _ = time.Parse(time.RFC3339Nano, av)
		out = append(out, m)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, m := range out {
		if _, err := tx.ExecContext(ctx, `UPDATE event_outbox SET claimed_until=?, attempts=attempts+1 WHERE id=? AND delivered_at IS NULL`, until.Format(time.RFC3339Nano), m.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Attempts++
	}
	return out, nil
}
func (o Outbox) Ack(ctx context.Context, id string) error {
	res, err := o.DB.ExecContext(ctx, `UPDATE event_outbox SET delivered_at=?,claimed_until=NULL,last_error='' WHERE id=? AND delivered_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("outbox message not pending: %s", id)
	}
	return nil
}
func (o Outbox) Fail(ctx context.Context, id string, cause error, retryAt time.Time) error {
	if retryAt.IsZero() {
		retryAt = time.Now().UTC().Add(time.Minute)
	}
	msg := "delivery failed"
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
	}
	_, err := o.DB.ExecContext(ctx, `UPDATE event_outbox SET available_at=?,claimed_until=NULL,last_error=? WHERE id=? AND delivered_at IS NULL`, retryAt.UTC().Format(time.RFC3339Nano), msg, id)
	return err
}
