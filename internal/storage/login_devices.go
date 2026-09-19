package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type LoginDeviceRepo struct {
	db *sql.DB
}

func NewLoginDeviceRepo(db *sql.DB) *LoginDeviceRepo {
	return &LoginDeviceRepo{db: db}
}

func (r *LoginDeviceRepo) Create(ctx context.Context, tokenHash string, userID int64, keep int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin login device transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := toMillis(time.Now())

	const insert = "INSERT INTO login_devices (token_hash, user_id, created_at, last_used_at) VALUES (?, ?, ?, ?)"
	if _, err := tx.ExecContext(ctx, insert, tokenHash, userID, now, now); err != nil {
		return fmt.Errorf("insert login device: %w", err)
	}

	const prune = `DELETE FROM login_devices WHERE user_id = ? AND token_hash NOT IN (
		SELECT token_hash FROM login_devices WHERE user_id = ? ORDER BY last_used_at DESC, created_at DESC LIMIT ?)`
	if _, err := tx.ExecContext(ctx, prune, userID, userID, keep); err != nil {
		return fmt.Errorf("prune login devices: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit login device: %w", err)
	}

	return nil
}

func (r *LoginDeviceRepo) Exists(ctx context.Context, tokenHash string, userID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM login_devices WHERE token_hash = ? AND user_id = ?)"

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, tokenHash, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check login device: %w", err)
	}

	return exists, nil
}

func (r *LoginDeviceRepo) Touch(ctx context.Context, tokenHash string) error {
	const query = "UPDATE login_devices SET last_used_at = ? WHERE token_hash = ?"

	if _, err := r.db.ExecContext(ctx, query, toMillis(time.Now()), tokenHash); err != nil {
		return fmt.Errorf("touch login device: %w", err)
	}

	return nil
}

func (r *LoginDeviceRepo) DeleteByUser(ctx context.Context, userID int64) error {
	const query = "DELETE FROM login_devices WHERE user_id = ?"

	if _, err := r.db.ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("delete user login devices: %w", err)
	}

	return nil
}

func (r *LoginDeviceRepo) DeleteUnusedSince(ctx context.Context, before time.Time) (int64, error) {
	const query = "DELETE FROM login_devices WHERE last_used_at <= ?"

	result, err := r.db.ExecContext(ctx, query, toMillis(before))
	if err != nil {
		return 0, fmt.Errorf("delete unused login devices: %w", err)
	}

	return rowsAffected(result)
}

func (r *LoginDeviceRepo) DeleteOrphaned(ctx context.Context) (int64, error) {
	const query = "DELETE FROM login_devices WHERE user_id NOT IN (SELECT id FROM users)"

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned login devices: %w", err)
	}

	return rowsAffected(result)
}
