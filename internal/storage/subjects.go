package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Subject struct {
	ID        int64
	Name      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SubjectRepo struct {
	db *sql.DB
}

func NewSubjectRepo(db *sql.DB) *SubjectRepo {
	return &SubjectRepo{db: db}
}

const subjectColumns = "id, name, active, created_at, updated_at"

func (r *SubjectRepo) List(ctx context.Context, includeInactive bool) ([]Subject, error) {
	query := "SELECT " + subjectColumns + " FROM subjects"
	if !includeInactive {
		query += " WHERE active = 1"
	}

	query += " ORDER BY name, id"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select subjects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var subjects []Subject

	for rows.Next() {
		subject, err := scanSubject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}

		subjects = append(subjects, subject)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subjects: %w", err)
	}

	return subjects, nil
}

func (r *SubjectRepo) ByID(ctx context.Context, id int64) (Subject, error) {
	const query = "SELECT " + subjectColumns + " FROM subjects WHERE id = ?"

	subject, err := scanSubject(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Subject{}, fmt.Errorf("select subject by id: %w", err)
	}

	return subject, nil
}

func (r *SubjectRepo) Create(ctx context.Context, name string) (int64, error) {
	const query = "INSERT INTO subjects (name, active, created_at, updated_at) VALUES (?, 1, ?, ?)"

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query, name, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert subject: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert subject: %w", err)
	}

	return id, nil
}

func (r *SubjectRepo) Update(ctx context.Context, id int64, name string) error {
	const query = "UPDATE subjects SET name = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, name, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("update subject: %w", err)
	}

	return requireAffected(result, "update subject")
}

func (r *SubjectRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const query = "UPDATE subjects SET active = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, active, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("set subject active: %w", err)
	}

	return requireAffected(result, "set subject active")
}

func (r *SubjectRepo) ActiveNameExists(ctx context.Context, name string, excludeID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM subjects WHERE active = 1 AND name = ? AND id != ?)"

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, name, excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check subject name: %w", err)
	}

	return exists, nil
}

func scanSubject(row scanner) (Subject, error) {
	var (
		subject   Subject
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&subject.ID, &subject.Name, &subject.Active, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Subject{}, ErrNotFound
	}
	if err != nil {
		return Subject{}, err
	}

	subject.CreatedAt = fromMillis(createdAt)
	subject.UpdatedAt = fromMillis(updatedAt)

	return subject, nil
}

func requireAffected(result sql.Result, operation string) error {
	count, err := rowsAffected(result)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	if count == 0 {
		return fmt.Errorf("%s: %w", operation, ErrNotFound)
	}

	return nil
}
