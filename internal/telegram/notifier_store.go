package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

type notificationDeliveryState string

const (
	deliveryPrepared  notificationDeliveryState = "prepared"
	deliverySending   notificationDeliveryState = "sending"
	deliveryApplied   notificationDeliveryState = "applied"
	deliveryAmbiguous notificationDeliveryState = "ambiguous"
)

type notificationRecipient struct {
	TelegramUserID int64
	UserID         string
	ChatID         int64
}

type notificationDelivery struct {
	Recipient         notificationRecipient
	State             notificationDeliveryState
	ProviderMessageID int64
}

type notificationOperation struct {
	IdempotencyKey string
	ExecutionID    string
	SemanticDigest string
	Deliveries     []notificationDelivery
}

type NotificationDeliveryStore struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s NotificationDeliveryStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s NotificationDeliveryStore) Freeze(
	ctx context.Context,
	op gateway.Operation,
	semanticDigest string,
	recipients []notificationRecipient,
) (notificationOperation, bool, error) {
	if s.DB == nil {
		return notificationOperation{}, false, errors.New("telegram notifier: delivery database unavailable")
	}
	if err := op.Validate(); err != nil {
		return notificationOperation{}, false, err
	}
	semanticDigest = strings.TrimSpace(semanticDigest)
	if semanticDigest == "" {
		return notificationOperation{}, false, errors.New("telegram notifier: semantic digest is required")
	}
	if len(recipients) == 0 {
		return notificationOperation{}, false, ErrNoNotificationRecipients
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return notificationOperation{}, false, err
	}
	defer tx.Rollback()
	now := s.now().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO telegram_notification_operations
		(idempotency_key,execution_id,semantic_digest,created_at,updated_at)
		VALUES(?,?,?,?,?)`,
		strings.TrimSpace(op.IdempotencyKey), strings.TrimSpace(op.ExecutionID), semanticDigest, now, now,
	)
	if err != nil {
		return notificationOperation{}, false, fmt.Errorf("telegram notifier: freeze operation: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return notificationOperation{}, false, err
	}
	created := inserted == 1
	if created {
		for _, recipient := range recipients {
			if recipient.TelegramUserID <= 0 || recipient.ChatID == 0 || strings.TrimSpace(recipient.UserID) == "" {
				return notificationOperation{}, false, errors.New("telegram notifier: invalid frozen recipient")
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO telegram_notification_deliveries
				(idempotency_key,telegram_user_id,user_id,chat_id,state,provider_message_id,created_at,updated_at)
				VALUES(?,?,?,?,?,NULL,?,?)`,
				strings.TrimSpace(op.IdempotencyKey), recipient.TelegramUserID, strings.TrimSpace(recipient.UserID), recipient.ChatID,
				string(deliveryPrepared), now, now,
			); err != nil {
				return notificationOperation{}, false, fmt.Errorf("telegram notifier: freeze recipient: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return notificationOperation{}, false, err
	}
	frozen, err := s.Load(ctx, strings.TrimSpace(op.IdempotencyKey))
	return frozen, created, err
}

func (s NotificationDeliveryStore) Load(ctx context.Context, idempotencyKey string) (notificationOperation, error) {
	if s.DB == nil {
		return notificationOperation{}, errors.New("telegram notifier: delivery database unavailable")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	var out notificationOperation
	err := s.DB.QueryRowContext(ctx, `
		SELECT idempotency_key,execution_id,semantic_digest
		FROM telegram_notification_operations WHERE idempotency_key=?`, idempotencyKey,
	).Scan(&out.IdempotencyKey, &out.ExecutionID, &out.SemanticDigest)
	if err != nil {
		return notificationOperation{}, err
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT telegram_user_id,user_id,chat_id,state,COALESCE(provider_message_id,0)
		FROM telegram_notification_deliveries
		WHERE idempotency_key=?
		ORDER BY telegram_user_id`, idempotencyKey,
	)
	if err != nil {
		return notificationOperation{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var delivery notificationDelivery
		var state string
		if err := rows.Scan(
			&delivery.Recipient.TelegramUserID,
			&delivery.Recipient.UserID,
			&delivery.Recipient.ChatID,
			&state,
			&delivery.ProviderMessageID,
		); err != nil {
			return notificationOperation{}, err
		}
		delivery.State = notificationDeliveryState(state)
		if !validDeliveryState(delivery.State) {
			return notificationOperation{}, fmt.Errorf("telegram notifier: invalid durable delivery state %q", state)
		}
		out.Deliveries = append(out.Deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		return notificationOperation{}, err
	}
	if len(out.Deliveries) == 0 {
		return notificationOperation{}, errors.New("telegram notifier: frozen operation has no recipients")
	}
	return out, nil
}

// ClaimPrepared performs the durable compare-and-swap that grants permission to
// call Telegram. claimed is true only for the caller that changed prepared to
// sending. Other concurrent callers must never send based solely on the loaded
// sending state.
func (s NotificationDeliveryStore) ClaimPrepared(
	ctx context.Context,
	idempotencyKey string,
	telegramUserID int64,
) (delivery notificationDelivery, claimed bool, err error) {
	if s.DB == nil {
		return notificationDelivery{}, false, errors.New("telegram notifier: delivery database unavailable")
	}
	now := s.now().Format(time.RFC3339Nano)
	result, err := s.DB.ExecContext(ctx, `
		UPDATE telegram_notification_deliveries
		SET state=?,updated_at=?
		WHERE idempotency_key=? AND telegram_user_id=? AND state=?`,
		string(deliverySending), now, strings.TrimSpace(idempotencyKey), telegramUserID, string(deliveryPrepared),
	)
	if err != nil {
		return notificationDelivery{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return notificationDelivery{}, false, err
	}
	delivery, err = s.loadDelivery(ctx, idempotencyKey, telegramUserID)
	if err != nil {
		return notificationDelivery{}, false, err
	}
	return delivery, affected == 1, nil
}

func (s NotificationDeliveryStore) MarkApplied(
	ctx context.Context,
	idempotencyKey string,
	telegramUserID int64,
	providerMessageID int64,
) error {
	if providerMessageID <= 0 {
		return errors.New("telegram notifier: provider message id is required")
	}
	return s.transition(ctx, idempotencyKey, telegramUserID, deliverySending, deliveryApplied, providerMessageID)
}

func (s NotificationDeliveryStore) MarkAmbiguous(ctx context.Context, idempotencyKey string, telegramUserID int64) error {
	return s.transition(ctx, idempotencyKey, telegramUserID, deliverySending, deliveryAmbiguous, 0)
}

func (s NotificationDeliveryStore) ReleasePrepared(ctx context.Context, idempotencyKey string, telegramUserID int64) error {
	return s.transition(ctx, idempotencyKey, telegramUserID, deliverySending, deliveryPrepared, 0)
}

func (s NotificationDeliveryStore) transition(
	ctx context.Context,
	idempotencyKey string,
	telegramUserID int64,
	from notificationDeliveryState,
	to notificationDeliveryState,
	providerMessageID int64,
) error {
	if s.DB == nil {
		return errors.New("telegram notifier: delivery database unavailable")
	}
	now := s.now().Format(time.RFC3339Nano)
	var result sql.Result
	var err error
	if providerMessageID > 0 {
		result, err = s.DB.ExecContext(ctx, `
			UPDATE telegram_notification_deliveries
			SET state=?,provider_message_id=?,updated_at=?
			WHERE idempotency_key=? AND telegram_user_id=? AND state=?`,
			string(to), providerMessageID, now, strings.TrimSpace(idempotencyKey), telegramUserID, string(from),
		)
	} else {
		result, err = s.DB.ExecContext(ctx, `
			UPDATE telegram_notification_deliveries
			SET state=?,updated_at=?
			WHERE idempotency_key=? AND telegram_user_id=? AND state=?`,
			string(to), now, strings.TrimSpace(idempotencyKey), telegramUserID, string(from),
		)
	}
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("telegram notifier: stale delivery transition %s->%s", from, to)
	}
	return nil
}

func (s NotificationDeliveryStore) loadDelivery(
	ctx context.Context,
	idempotencyKey string,
	telegramUserID int64,
) (notificationDelivery, error) {
	var delivery notificationDelivery
	var state string
	err := s.DB.QueryRowContext(ctx, `
		SELECT telegram_user_id,user_id,chat_id,state,COALESCE(provider_message_id,0)
		FROM telegram_notification_deliveries
		WHERE idempotency_key=? AND telegram_user_id=?`,
		strings.TrimSpace(idempotencyKey), telegramUserID,
	).Scan(
		&delivery.Recipient.TelegramUserID,
		&delivery.Recipient.UserID,
		&delivery.Recipient.ChatID,
		&state,
		&delivery.ProviderMessageID,
	)
	if err != nil {
		return notificationDelivery{}, err
	}
	delivery.State = notificationDeliveryState(state)
	if !validDeliveryState(delivery.State) {
		return notificationDelivery{}, fmt.Errorf("telegram notifier: invalid durable delivery state %q", state)
	}
	return delivery, nil
}

func validDeliveryState(state notificationDeliveryState) bool {
	switch state {
	case deliveryPrepared, deliverySending, deliveryApplied, deliveryAmbiguous:
		return true
	default:
		return false
	}
}
