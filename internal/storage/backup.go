package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
)

func BackupDatabase(ctx context.Context, path, dst string) error {
	db, err := sql.Open("sqlite", readOnlyDSN(path))
	if err != nil {
		return fmt.Errorf("open database for backup: %w", err)
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}

	return nil
}

func readOnlyDSN(path string) string {
	params := url.Values{}
	params.Add("mode", "ro")
	params.Add("_pragma", "busy_timeout(5000)")

	return "file:" + path + "?" + params.Encode()
}
