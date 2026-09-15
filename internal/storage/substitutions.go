package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Substitution struct {
	ID        int64
	ClassID   int64
	SubjectID int64
	TeacherID int64
	StartDate time.Time
	EndDate   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SubstitutionRepo struct {
	db *sql.DB
}

func NewSubstitutionRepo(db *sql.DB) *SubstitutionRepo {
	return &SubstitutionRepo{db: db}
}

const (
	substitutionColumns = "id, class_id, subject_id, teacher_id, start_date, end_date, created_at, updated_at"
	openEndDate         = "9999-12-31"
)

func (r *SubstitutionRepo) ListByYear(ctx context.Context, year int, today time.Time, includeEnded bool) ([]Substitution, error) {
	query := "SELECT " + substitutionColumns + " FROM substitutions WHERE class_id IN (SELECT id FROM classes WHERE year = ?)"
	args := []any{year}

	if !includeEnded {
		query += " AND COALESCE(end_date, ?) >= ?"
		args = append(args, openEndDate, toDate(today))
	}

	query += " ORDER BY start_date DESC, id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select substitutions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return collectSubstitutions(rows)
}

func (r *SubstitutionRepo) ListActiveByTeacher(ctx context.Context, teacherID int64, today time.Time) ([]Substitution, error) {
	const query = "SELECT " + substitutionColumns + " FROM substitutions WHERE teacher_id = ? AND COALESCE(end_date, ?) >= ? ORDER BY id"

	rows, err := r.db.QueryContext(ctx, query, teacherID, openEndDate, toDate(today))
	if err != nil {
		return nil, fmt.Errorf("select teacher substitutions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return collectSubstitutions(rows)
}

func (r *SubstitutionRepo) ByID(ctx context.Context, id int64) (Substitution, error) {
	const query = "SELECT " + substitutionColumns + " FROM substitutions WHERE id = ?"

	substitution, err := scanSubstitution(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Substitution{}, fmt.Errorf("select substitution by id: %w", err)
	}

	return substitution, nil
}

func (r *SubstitutionRepo) Create(ctx context.Context, substitution Substitution) (int64, error) {
	const query = `INSERT INTO substitutions (class_id, subject_id, teacher_id, start_date, end_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query,
		substitution.ClassID, substitution.SubjectID, substitution.TeacherID,
		toDate(substitution.StartDate), toNullDate(substitution.EndDate), now, now)
	if err != nil {
		return 0, fmt.Errorf("insert substitution: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert substitution: %w", err)
	}

	return id, nil
}

func (r *SubstitutionRepo) UpdatePeriod(ctx context.Context, id int64, start, end time.Time) error {
	const query = "UPDATE substitutions SET start_date = ?, end_date = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, toDate(start), toNullDate(end), toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("update substitution period: %w", err)
	}

	return requireAffected(result, "update substitution period")
}

func (r *SubstitutionRepo) Delete(ctx context.Context, id int64) error {
	const query = "DELETE FROM substitutions WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete substitution: %w", err)
	}

	return requireAffected(result, "delete substitution")
}

func (r *SubstitutionRepo) Overlaps(ctx context.Context, classID, subjectID int64, start, end time.Time, excludeID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM substitutions
		WHERE class_id = ? AND subject_id = ? AND id != ?
			AND start_date <= COALESCE(?, ?) AND COALESCE(end_date, ?) >= ?)`

	var exists bool

	err := r.db.QueryRowContext(ctx, query, classID, subjectID, excludeID,
		toNullDate(end), openEndDate, openEndDate, toDate(start)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check substitution overlap: %w", err)
	}

	return exists, nil
}

func collectSubstitutions(rows *sql.Rows) ([]Substitution, error) {
	var substitutions []Substitution

	for rows.Next() {
		substitution, err := scanSubstitution(rows)
		if err != nil {
			return nil, fmt.Errorf("scan substitution: %w", err)
		}

		substitutions = append(substitutions, substitution)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate substitutions: %w", err)
	}

	return substitutions, nil
}

func scanSubstitution(row scanner) (Substitution, error) {
	var (
		substitution Substitution
		start        string
		end          sql.NullString
		createdAt    int64
		updatedAt    int64
	)

	err := row.Scan(&substitution.ID, &substitution.ClassID, &substitution.SubjectID, &substitution.TeacherID,
		&start, &end, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Substitution{}, ErrNotFound
	}
	if err != nil {
		return Substitution{}, err
	}

	if substitution.StartDate, err = fromDate(start); err != nil {
		return Substitution{}, err
	}

	if substitution.EndDate, err = fromNullDate(end); err != nil {
		return Substitution{}, err
	}

	substitution.CreatedAt = fromMillis(createdAt)
	substitution.UpdatedAt = fromMillis(updatedAt)

	return substitution, nil
}
