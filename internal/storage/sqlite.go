package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/storage/migrations"
)

var ErrNotFound = errors.New("not found")

func Open(ctx context.Context, cfg config.DB) (*sql.DB, error) {
	if dir := filepath.Dir(cfg.Path); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dsn(cfg.Path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

func dsn(path string) string {
	params := url.Values{}
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_txlock", "immediate")

	return "file:" + path + "?" + params.Encode()
}

func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
