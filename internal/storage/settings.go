package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SettingRepo struct {
	db *sql.DB
}

func NewSettingRepo(db *sql.DB) *SettingRepo {
	return &SettingRepo{db: db}
}

func (r *SettingRepo) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("select settings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	values := map[string]string{}

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}

		values[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}

	return values, nil
}

func (r *SettingRepo) Set(ctx context.Context, values map[string]string) error {
	const query = `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin settings: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := toMillis(time.Now())

	for key, value := range values {
		if _, err := tx.ExecContext(ctx, query, key, value, now); err != nil {
			return fmt.Errorf("upsert setting %s: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit settings: %w", err)
	}

	return nil
}
