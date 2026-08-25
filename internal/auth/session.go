package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

type Session struct {
	ID        string
	UserID    string
	Token     string
	CSRF      string
	CreatedAt time.Time
	ExpiresAt time.Time
}
type SessionStore struct {
	db  *sql.DB
	ttl time.Duration
}

func NewSessionStore(db *sql.DB, ttl time.Duration) *SessionStore {
	return &SessionStore{db: db, ttl: ttl}
}
func randomToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	s := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(s))
	return s, sum[:], nil
}
func (s *SessionStore) Create(ctx context.Context, userID string) (Session, error) {
	if s.ttl <= 0 {
		s.ttl = 12 * time.Hour
	}
	id, err := domain.NewID("ses")
	if err != nil {
		return Session{}, err
	}
	token, th, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, ch, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	exp := now.Add(s.ttl)
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(id,user_id,token_hash,csrf_hash,created_at,expires_at,last_seen_at,reauthenticated_at) VALUES(?,?,?,?,?,?,?,?)`, id.String(), userID, th, ch, now.Format(time.RFC3339Nano), exp.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Session{}, err
	}
	return Session{ID: id.String(), UserID: userID, Token: token, CSRF: csrf, CreatedAt: now, ExpiresAt: exp}, nil
}

type Principal struct {
	User              User
	SessionID         string
	ExpiresAt         time.Time
	ReauthenticatedAt time.Time
}

func (s *SessionStore) Resolve(ctx context.Context, token string) (Principal, error) {
	sum := sha256.Sum256([]byte(token))
	var p Principal
	var role, exp, reauth string
	var disabled int
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.role,u.disabled,s.id,s.expires_at,COALESCE(s.reauthenticated_at,s.created_at) FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.revoked_at IS NULL`, sum[:]).Scan(&p.User.ID, &p.User.Username, &role, &disabled, &p.SessionID, &exp, &reauth)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, errors.New("invalid session")
	}
	if err != nil {
		return Principal{}, err
	}
	p.ExpiresAt, _ = time.Parse(time.RFC3339Nano, exp)
	p.ReauthenticatedAt, _ = time.Parse(time.RFC3339Nano, reauth)
	if disabled != 0 || time.Now().UTC().After(p.ExpiresAt) {
		return Principal{}, errors.New("invalid session")
	}
	p.User.Role = Role(role)
	_, _ = s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), p.SessionID)
	return p, nil
}
func (s *SessionStore) ValidateCSRF(ctx context.Context, sessionID, csrf string) bool {
	sum := sha256.Sum256([]byte(csrf))
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id=? AND csrf_hash=? AND revoked_at IS NULL`, sessionID, sum[:]).Scan(&n)
	return err == nil && n == 1
}
func (s *SessionStore) Revoke(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), sessionID)
	return err
}

func (s *SessionStore) MarkReauthenticated(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("session id required")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE sessions SET reauthenticated_at=? WHERE id=? AND revoked_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), sessionID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("session unavailable")
	}
	return nil
}

// RotateCSRF invalidates the previous CSRF token for an authenticated session
// and returns a fresh token. Only the SHA-256 digest is persisted.
func (s *SessionStore) RotateCSRF(ctx context.Context, sessionID string) (string, error) {
	if s == nil || s.db == nil || sessionID == "" {
		return "", errors.New("session unavailable")
	}
	token, hash, err := randomToken()
	if err != nil {
		return "", err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE sessions SET csrf_hash=? WHERE id=? AND revoked_at IS NULL`, hash, sessionID)
	if err != nil {
		return "", err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if n != 1 {
		return "", errors.New("session unavailable")
	}
	return token, nil
}
