package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type ParentChild struct {
	ParentID  int64
	StudentID int64
}

type ParentChildRepo struct {
	db *sql.DB
}

func NewParentChildRepo(db *sql.DB) *ParentChildRepo {
	return &ParentChildRepo{db: db}
}

func (r *ParentChildRepo) ListAll(ctx context.Context) ([]ParentChild, error) {
	const query = "SELECT parent_id, student_id FROM parent_children"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select parent children: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var links []ParentChild

	for rows.Next() {
		var link ParentChild
		if err := rows.Scan(&link.ParentID, &link.StudentID); err != nil {
			return nil, fmt.Errorf("scan parent child: %w", err)
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate parent children: %w", err)
	}

	return links, nil
}

func (r *ParentChildRepo) Add(ctx context.Context, parentID, studentID int64) error {
	const query = "INSERT OR IGNORE INTO parent_children (parent_id, student_id) VALUES (?, ?)"

	if _, err := r.db.ExecContext(ctx, query, parentID, studentID); err != nil {
		return fmt.Errorf("add child to parent: %w", err)
	}

	return nil
}

func (r *ParentChildRepo) Remove(ctx context.Context, parentID, studentID int64) error {
	const query = "DELETE FROM parent_children WHERE parent_id = ? AND student_id = ?"

	result, err := r.db.ExecContext(ctx, query, parentID, studentID)
	if err != nil {
		return fmt.Errorf("remove child from parent: %w", err)
	}

	return requireAffected(result, "remove child from parent")
}
