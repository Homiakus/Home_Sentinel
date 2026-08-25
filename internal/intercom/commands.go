package intercom

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type MQTTPublisher interface {
	Publish(context.Context, mqttint.Message) error
}

type CommandRecord struct {
	RequestID      string    `json:"request_id"`
	DeviceID       string    `json:"device_id"`
	ActorID        string    `json:"actor_id"`
	CorrelationID  string    `json:"correlation_id"`
	Action         string    `json:"action"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Status         string    `json:"status"`
	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
	CompletedAt    time.Time `json:"completed_at,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type CommandStore struct{ DB *sql.DB }

func (s CommandStore) Create(ctx context.Context, r CommandRecord) error {
	if s.DB == nil {
		return errors.New("intercom command database unavailable")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO intercom_commands(request_id,device_id,actor_id,correlation_id,action,issued_at,expires_at,status,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, r.RequestID, r.DeviceID, r.ActorID, r.CorrelationID, r.Action, r.IssuedAt.Format(time.RFC3339Nano), r.ExpiresAt.Format(time.RFC3339Nano), r.Status, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s CommandStore) UpdateAck(ctx context.Context, deviceID string, ack Ack, now time.Time) error {
	status := "acknowledged"
	errText := ""
	if !ack.Accepted {
		status = "rejected"
		errText = ack.Reason
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE intercom_commands SET status=?,acknowledged_at=?,error=?,updated_at=? WHERE request_id=? AND device_id=? AND status='pending' AND expires_at>=?`, status, now.Format(time.RFC3339Nano), errText, now.Format(time.RFC3339Nano), ack.RequestID, deviceID, now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("intercom ack rejected: command missing, stale or already acknowledged")
	}
	return nil
}
func (s CommandStore) UpdateResult(ctx context.Context, deviceID string, result Result, now time.Time) error {
	status := "completed"
	errText := result.Error
	if !result.Success {
		status = "rejected"
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE intercom_commands SET status=?,completed_at=?,error=?,updated_at=? WHERE request_id=? AND device_id=? AND status IN ('pending','acknowledged') AND expires_at>=?`, status, now.Format(time.RFC3339Nano), errText, now.Format(time.RFC3339Nano), result.RequestID, deviceID, now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("intercom result rejected: command missing, stale or already completed")
	}
	return nil
}
func (s CommandStore) MarkPublishFailed(ctx context.Context, requestID string, cause error) {
	if s.DB == nil {
		return
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE intercom_commands SET status='publish_failed',error=?,updated_at=? WHERE request_id=? AND status='pending'`, cause.Error(), time.Now().UTC().Format(time.RFC3339Nano), requestID)
}
func (s CommandStore) Get(ctx context.Context, id string) (CommandRecord, error) {
	var r CommandRecord
	var issued, expires, ack, done sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT request_id,device_id,actor_id,correlation_id,action,issued_at,expires_at,status,acknowledged_at,completed_at,error FROM intercom_commands WHERE request_id=?`, id).Scan(&r.RequestID, &r.DeviceID, &r.ActorID, &r.CorrelationID, &r.Action, &issued, &expires, &r.Status, &ack, &done, &r.Error)
	if errors.Is(err, sql.ErrNoRows) {
		return r, repository.ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.IssuedAt, _ = time.Parse(time.RFC3339Nano, issued.String)
	r.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires.String)
	if ack.Valid {
		r.AcknowledgedAt, _ = time.Parse(time.RFC3339Nano, ack.String)
	}
	if done.Valid {
		r.CompletedAt, _ = time.Parse(time.RFC3339Nano, done.String)
	}
	return r, nil
}

type UnlockRequest struct {
	DeviceID, ActorID, CorrelationID string
	TTL                              time.Duration
}

func (s *Service) Unlock(ctx context.Context, in UnlockRequest) (CommandRecord, error) {
	if s == nil || s.MQTT == nil {
		return CommandRecord{}, errors.New("intercom MQTT unavailable")
	}
	dev, err := s.Get(ctx, in.DeviceID)
	if err != nil {
		return CommandRecord{}, err
	}
	if !dev.Capabilities.Lock {
		return CommandRecord{}, errors.New("intercom has no lock capability")
	}
	if in.ActorID == "" {
		return CommandRecord{}, errors.New("actor id required")
	}
	corr := domain.ID(in.CorrelationID)
	if !corr.ValidFor("cor") {
		return CommandRecord{}, errors.New("valid correlation id required")
	}
	if in.TTL <= 0 {
		in.TTL = 5 * time.Second
	}
	if in.TTL > 15*time.Second {
		return CommandRecord{}, errors.New("unlock TTL exceeds 15 seconds")
	}
	requestID, err := domain.NewID("cmd")
	if err != nil {
		return CommandRecord{}, err
	}
	now := s.now()
	expires := now.Add(in.TTL)
	record := CommandRecord{RequestID: requestID.String(), DeviceID: dev.ID, ActorID: in.ActorID, CorrelationID: in.CorrelationID, Action: "unlock", IssuedAt: now, ExpiresAt: expires, Status: "pending"}
	if err := s.Commands.Create(ctx, record); err != nil {
		return CommandRecord{}, err
	}
	payload, err := json.Marshal(Command{SchemaVersion: SchemaVersion, RequestID: record.RequestID, CorrelationID: record.CorrelationID, Action: "unlock", IssuedAt: now, ExpiresAt: expires})
	if err != nil {
		return CommandRecord{}, err
	}
	topic, err := mqttint.IntercomCommand(dev.ID, "unlock")
	if err != nil {
		return CommandRecord{}, err
	}
	if err := s.MQTT.Publish(ctx, mqttint.Message{Topic: topic, Payload: payload, QoS: 1, Retained: false}); err != nil {
		s.Commands.MarkPublishFailed(ctx, record.RequestID, err)
		return CommandRecord{}, fmt.Errorf("publish unlock command: %w", err)
	}
	if s.Audit != nil {
		_, _ = s.Audit.Append(ctx, repository.AuditEntry{Actor: in.ActorID, Source: "sentinel", Action: "door.unlock.request", Target: "intercom:" + dev.ID, Result: "published", RequestID: record.RequestID, CorrelationID: record.CorrelationID})
	}
	return record, nil
}
