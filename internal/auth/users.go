package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
)

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

func (r Role) Valid() bool { return r == RoleViewer || r == RoleOperator || r == RoleAdmin }

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type UserStore struct{ db *sql.DB }

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }
func (s *UserStore) Create(ctx context.Context, username, password string, role Role) (User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 {
		return User{}, errors.New("username must be at least 3 characters")
	}
	if !role.Valid() {
		return User{}, errors.New("invalid role")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	id, err := domain.NewID("usr")
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO users(id,username,password_hash,role,created_at,updated_at) VALUES(?,?,?,?,?,?)`, id.String(), username, hash, string(role), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return User{ID: id.String(), Username: username, Role: role, CreatedAt: now, UpdatedAt: now}, nil
}
func (s *UserStore) Authenticate(ctx context.Context, username, password string) (User, error) {
	var u User
	var role, cr, ur, hash string
	var disabled int
	err := s.db.QueryRowContext(ctx, `SELECT id,username,password_hash,role,disabled,created_at,updated_at FROM users WHERE username=?`, strings.TrimSpace(username)).Scan(&u.ID, &u.Username, &hash, &role, &disabled, &cr, &ur)
	if errors.Is(err, sql.ErrNoRows) {
		dummy, _ := HashPassword("dummy-password-value")
		_ = VerifyPassword(dummy, password)
		return User{}, errors.New("invalid credentials")
	}
	if err != nil {
		return User{}, err
	}
	if !VerifyPassword(hash, password) || disabled != 0 {
		return User{}, errors.New("invalid credentials")
	}
	u.Role = Role(role)
	u.Disabled = false
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, cr)
	u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, ur)
	return u, nil
}
func (s *UserStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *UserStore) GetByID(ctx context.Context, id string) (User, error) {
	var u User
	var role, cr, ur string
	var disabled int
	err := s.db.QueryRowContext(ctx, `SELECT id,username,role,disabled,created_at,updated_at FROM users WHERE id=?`, id).Scan(&u.ID, &u.Username, &role, &disabled, &cr, &ur)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, errors.New("user not found")
	}
	if err != nil {
		return User{}, err
	}
	u.Role = Role(role)
	u.Disabled = disabled != 0
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, cr)
	u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, ur)
	return u, nil
}

var ErrLastEnabledAdmin = errors.New("cannot remove the last enabled administrator")

func (s *UserStore) List(ctx context.Context, limit int) ([]User, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,username,role,disabled,created_at,updated_at FROM users ORDER BY username LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]User, 0)
	for rows.Next() {
		var u User
		var role, created, updated string
		var disabled int
		if err := rows.Scan(&u.ID, &u.Username, &role, &disabled, &created, &updated); err != nil {
			return nil, err
		}
		u.Role = Role(role)
		u.Disabled = disabled != 0
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateAccess atomically changes a user's role/disabled state while preserving
// the invariant that at least one enabled administrator always exists.
func (s *UserStore) UpdateAccess(ctx context.Context, id string, role Role, disabled bool) (User, error) {
	if id == "" || !role.Valid() {
		return User{}, errors.New("user id and valid role required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var currentRole, username, created string
	var currentDisabled int
	if err := tx.QueryRowContext(ctx, `SELECT username,role,disabled,created_at FROM users WHERE id=?`, id).Scan(&username, &currentRole, &currentDisabled, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, errors.New("user not found")
		}
		return User{}, err
	}
	if Role(currentRole) == RoleAdmin && currentDisabled == 0 && (role != RoleAdmin || disabled) {
		var admins int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role=? AND disabled=0`, string(RoleAdmin)).Scan(&admins); err != nil {
			return User{}, err
		}
		if admins <= 1 {
			return User{}, ErrLastEnabledAdmin
		}
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role=?,disabled=?,updated_at=? WHERE id=?`, string(role), boolToInt(disabled), now.Format(time.RFC3339Nano), id); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, created)
	return User{ID: id, Username: username, Role: role, Disabled: disabled, CreatedAt: createdAt, UpdatedAt: now}, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
