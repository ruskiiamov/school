package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Class struct {
	ID        int64
	Year      int
	Name      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ClassRepo struct {
	db *sql.DB
}

func NewClassRepo(db *sql.DB) *ClassRepo {
	return &ClassRepo{db: db}
}

const classColumns = "id, year, name, active, created_at, updated_at"

func (r *ClassRepo) ListByYear(ctx context.Context, year int, includeInactive bool) ([]Class, error) {
	query := "SELECT " + classColumns + " FROM classes WHERE year = ?"
	if !includeInactive {
		query += " AND active = 1"
	}

	query += " ORDER BY CAST(name AS INTEGER), name, id"

	rows, err := r.db.QueryContext(ctx, query, year)
	if err != nil {
		return nil, fmt.Errorf("select classes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var classes []Class

	for rows.Next() {
		class, err := scanClass(rows)
		if err != nil {
			return nil, fmt.Errorf("scan class: %w", err)
		}

		classes = append(classes, class)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate classes: %w", err)
	}

	return classes, nil
}

func (r *ClassRepo) Years(ctx context.Context) ([]int, error) {
	const query = "SELECT DISTINCT year FROM classes ORDER BY year"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select class years: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var years []int

	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, fmt.Errorf("scan class year: %w", err)
		}

		years = append(years, year)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate class years: %w", err)
	}

	return years, nil
}

func (r *ClassRepo) ByID(ctx context.Context, id int64) (Class, error) {
	const query = "SELECT " + classColumns + " FROM classes WHERE id = ?"

	class, err := scanClass(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Class{}, fmt.Errorf("select class by id: %w", err)
	}

	return class, nil
}

func (r *ClassRepo) Create(ctx context.Context, year int, name string) (int64, error) {
	const query = "INSERT INTO classes (year, name, active, created_at, updated_at) VALUES (?, ?, 1, ?, ?)"

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query, year, name, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert class: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert class: %w", err)
	}

	return id, nil
}

func (r *ClassRepo) Update(ctx context.Context, id int64, name string) error {
	const query = "UPDATE classes SET name = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, name, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("update class: %w", err)
	}

	return requireAffected(result, "update class")
}

func (r *ClassRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const query = "UPDATE classes SET active = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, active, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("set class active: %w", err)
	}

	return requireAffected(result, "set class active")
}

type NewClass struct {
	Name       string
	StudentIDs []int64
}

func (r *ClassRepo) Transfer(ctx context.Context, year int, classes []NewClass) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := toMillis(time.Now())

	for _, class := range classes {
		const insert = "INSERT INTO classes (year, name, active, created_at, updated_at) VALUES (?, ?, 1, ?, ?)"

		result, err := tx.ExecContext(ctx, insert, year, class.Name, now, now)
		if err != nil {
			return fmt.Errorf("insert class: %w", err)
		}

		classID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("insert class: %w", err)
		}

		for _, studentID := range class.StudentIDs {
			const member = "INSERT INTO class_students (class_id, student_id) VALUES (?, ?)"

			if _, err := tx.ExecContext(ctx, member, classID, studentID); err != nil {
				return fmt.Errorf("add student to class: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *ClassRepo) CountActiveByYear(ctx context.Context, year int) (int, error) {
	const query = "SELECT COUNT(*) FROM classes WHERE year = ? AND active = 1"

	var count int
	if err := r.db.QueryRowContext(ctx, query, year).Scan(&count); err != nil {
		return 0, fmt.Errorf("count classes: %w", err)
	}

	return count, nil
}

func (r *ClassRepo) NameExists(ctx context.Context, year int, name string, excludeID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM classes WHERE year = ? AND name = ? AND id != ?)"

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, year, name, excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check class name: %w", err)
	}

	return exists, nil
}

func scanClass(row scanner) (Class, error) {
	var (
		class     Class
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&class.ID, &class.Year, &class.Name, &class.Active, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Class{}, ErrNotFound
	}
	if err != nil {
		return Class{}, err
	}

	class.CreatedAt = fromMillis(createdAt)
	class.UpdatedAt = fromMillis(updatedAt)

	return class, nil
}
