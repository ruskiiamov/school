package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Session struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, session Session) error {
	const query = "INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)"

	_, err := r.db.ExecContext(ctx, query, session.ID, session.UserID,
		toMillis(time.Now()), toMillis(session.ExpiresAt))
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

func (r *SessionRepo) ByID(ctx context.Context, id string) (Session, User, error) {
	const query = `SELECT s.id, s.user_id, s.created_at, s.expires_at,
		u.id, u.login, u.password_hash, u.full_name, u.role, u.active, u.created_at, u.updated_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = ?`

	var (
		session          Session
		user             User
		sessionCreatedAt int64
		expiresAt        int64
		userCreatedAt    int64
		userUpdatedAt    int64
	)

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID, &session.UserID, &sessionCreatedAt, &expiresAt,
		&user.ID, &user.Login, &user.PasswordHash, &user.FullName, &user.Role, &user.Active,
		&userCreatedAt, &userUpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, User{}, ErrNotFound
	}
	if err != nil {
		return Session{}, User{}, fmt.Errorf("select session: %w", err)
	}

	session.CreatedAt = fromMillis(sessionCreatedAt)
	session.ExpiresAt = fromMillis(expiresAt)
	user.CreatedAt = fromMillis(userCreatedAt)
	user.UpdatedAt = fromMillis(userUpdatedAt)

	return session, user, nil
}

func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	const query = "DELETE FROM sessions WHERE id = ?"

	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (r *SessionRepo) DeleteByUser(ctx context.Context, userID int64) error {
	const query = "DELETE FROM sessions WHERE user_id = ?"

	if _, err := r.db.ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}

	return nil
}

func (r *SessionRepo) DeleteByUserExcept(ctx context.Context, userID int64, keepID string) error {
	const query = "DELETE FROM sessions WHERE user_id = ? AND id != ?"

	if _, err := r.db.ExecContext(ctx, query, userID, keepID); err != nil {
		return fmt.Errorf("delete other user sessions: %w", err)
	}

	return nil
}

func (r *SessionRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	const query = "DELETE FROM sessions WHERE expires_at <= ?"

	result, err := r.db.ExecContext(ctx, query, toMillis(now))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}

	return rowsAffected(result)
}

func (r *SessionRepo) DeleteOrphaned(ctx context.Context) (int64, error) {
	const query = "DELETE FROM sessions WHERE user_id NOT IN (SELECT id FROM users)"

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned sessions: %w", err)
	}

	return rowsAffected(result)
}

func rowsAffected(result sql.Result) (int64, error) {
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return count, nil
}
