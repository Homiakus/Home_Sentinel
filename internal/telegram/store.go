package telegram

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type Binding struct {
	TelegramUserID int64     `json:"telegram_user_id"`
	UserID         string    `json:"user_id"`
	ChatID         int64     `json:"chat_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type PairingStore struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s PairingStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func randomOpaque(bytesN int) (string, []byte, error) {
	b := make([]byte, bytesN)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}
func (s PairingStore) Create(ctx context.Context, userID string, ttl time.Duration) (string, time.Time, error) {
	if s.DB == nil {
		return "", time.Time{}, errors.New("Telegram pairing database unavailable")
	}
	if strings.TrimSpace(userID) == "" {
		return "", time.Time{}, errors.New("user id required")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if ttl > time.Hour {
		return "", time.Time{}, errors.New("pairing TTL too long")
	}
	code, hash, err := randomOpaque(9)
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.now()
	exp := now.Add(ttl)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO telegram_pairings(code_hash,user_id,created_at,expires_at) VALUES(?,?,?,?)`, hash, userID, now.Format(time.RFC3339Nano), exp.Format(time.RFC3339Nano))
	return code, exp, err
}
func (s PairingStore) Consume(ctx context.Context, code string, telegramUserID, chatID int64) (Binding, error) {
	if telegramUserID <= 0 || chatID == 0 {
		return Binding{}, errors.New("Telegram identity invalid")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	now := s.now()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Binding{}, err
	}
	defer tx.Rollback()
	var userID, exp string
	var used sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT user_id,expires_at,used_at FROM telegram_pairings WHERE code_hash=?`, sum[:]).Scan(&userID, &exp, &used); errors.Is(err, sql.ErrNoRows) {
		return Binding{}, repository.ErrNotFound
	} else if err != nil {
		return Binding{}, err
	}
	expires, _ := time.Parse(time.RFC3339Nano, exp)
	if used.Valid || now.After(expires) {
		return Binding{}, errors.New("Telegram pairing code expired or already used")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE telegram_pairings SET used_at=? WHERE code_hash=? AND used_at IS NULL`, now.Format(time.RFC3339Nano), sum[:]); err != nil {
		return Binding{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_bindings(telegram_user_id,user_id,chat_id,created_at,revoked_at) VALUES(?,?,?,?,NULL) ON CONFLICT(telegram_user_id) DO UPDATE SET user_id=excluded.user_id,chat_id=excluded.chat_id,created_at=excluded.created_at,revoked_at=NULL`, telegramUserID, userID, chatID, now.Format(time.RFC3339Nano)); err != nil {
		return Binding{}, err
	}
	if err := tx.Commit(); err != nil {
		return Binding{}, err
	}
	return Binding{TelegramUserID: telegramUserID, UserID: userID, ChatID: chatID, CreatedAt: now}, nil
}
func (s PairingStore) Binding(ctx context.Context, telegramUserID int64) (Binding, error) {
	var b Binding
	var created string
	err := s.DB.QueryRowContext(ctx, `SELECT telegram_user_id,user_id,chat_id,created_at FROM telegram_bindings WHERE telegram_user_id=? AND revoked_at IS NULL`, telegramUserID).Scan(&b.TelegramUserID, &b.UserID, &b.ChatID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return b, repository.ErrNotFound
	}
	if err != nil {
		return b, err
	}
	b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return b, nil
}
func (s PairingStore) Revoke(ctx context.Context, telegramUserID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE telegram_bindings SET revoked_at=? WHERE telegram_user_id=?`, s.now().Format(time.RFC3339Nano), telegramUserID)
	return err
}

func (s PairingStore) ListBindings(ctx context.Context) ([]Binding, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT telegram_user_id,user_id,chat_id,created_at FROM telegram_bindings WHERE revoked_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Binding
	for rows.Next() {
		var b Binding
		var created string
		if err := rows.Scan(&b.TelegramUserID, &b.UserID, &b.ChatID, &created); err != nil {
			return nil, err
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, b)
	}
	return out, rows.Err()
}
