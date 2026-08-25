package telegram

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type PendingAction struct {
	Token          string    `json:"token,omitempty"`
	TelegramUserID int64     `json:"telegram_user_id"`
	UserID         string    `json:"user_id"`
	Action         string    `json:"action"`
	Target         string    `json:"target"`
	CorrelationID  string    `json:"correlation_id"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type ActionStore struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s ActionStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s ActionStore) Create(ctx context.Context, b Binding, action, target string, correlationID domain.ID, ttl time.Duration) (PendingAction, error) {
	if s.DB == nil {
		return PendingAction{}, errors.New("Telegram action database unavailable")
	}
	if b.TelegramUserID <= 0 || b.UserID == "" {
		return PendingAction{}, errors.New("bound Telegram user required")
	}
	if action == "" || target == "" {
		return PendingAction{}, errors.New("Telegram action and target required")
	}
	if !correlationID.ValidFor("cor") {
		return PendingAction{}, errors.New("valid correlation id required")
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	if ttl > 5*time.Minute {
		return PendingAction{}, errors.New("Telegram action TTL too long")
	}
	token, hash, err := randomOpaque(18)
	if err != nil {
		return PendingAction{}, err
	}
	now := s.now()
	exp := now.Add(ttl)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO telegram_actions(token_hash,telegram_user_id,user_id,action,target,correlation_id,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?)`, hash, b.TelegramUserID, b.UserID, action, target, correlationID.String(), now.Format(time.RFC3339Nano), exp.Format(time.RFC3339Nano))
	if err != nil {
		return PendingAction{}, err
	}
	return PendingAction{Token: token, TelegramUserID: b.TelegramUserID, UserID: b.UserID, Action: action, Target: target, CorrelationID: correlationID.String(), ExpiresAt: exp}, nil
}
func (s ActionStore) Consume(ctx context.Context, telegramUserID int64, token string) (PendingAction, error) {
	if telegramUserID <= 0 || strings.TrimSpace(token) == "" {
		return PendingAction{}, errors.New("Telegram action identity/token required")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	now := s.now()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return PendingAction{}, err
	}
	defer tx.Rollback()
	var p PendingAction
	var exp string
	var used sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT telegram_user_id,user_id,action,target,correlation_id,expires_at,used_at FROM telegram_actions WHERE token_hash=?`, sum[:]).Scan(&p.TelegramUserID, &p.UserID, &p.Action, &p.Target, &p.CorrelationID, &exp, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingAction{}, repository.ErrNotFound
	}
	if err != nil {
		return PendingAction{}, err
	}
	p.ExpiresAt, _ = time.Parse(time.RFC3339Nano, exp)
	if p.TelegramUserID != telegramUserID {
		return PendingAction{}, errors.New("Telegram action belongs to another user")
	}
	if used.Valid || now.After(p.ExpiresAt) {
		return PendingAction{}, errors.New("Telegram action expired or already used")
	}
	res, err := tx.ExecContext(ctx, `UPDATE telegram_actions SET used_at=? WHERE token_hash=? AND used_at IS NULL`, now.Format(time.RFC3339Nano), sum[:])
	if err != nil {
		return PendingAction{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return PendingAction{}, errors.New("Telegram action already consumed")
	}
	if err := tx.Commit(); err != nil {
		return PendingAction{}, err
	}
	return p, nil
}
