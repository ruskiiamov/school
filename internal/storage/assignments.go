package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type Assignment struct {
	ID        int64
	ClassID   int64
	SubjectID int64
	TeacherID int64
}

type AssignmentRepo struct {
	db *sql.DB
}

func NewAssignmentRepo(db *sql.DB) *AssignmentRepo {
	return &AssignmentRepo{db: db}
}

func (r *AssignmentRepo) ListByClass(ctx context.Context, classID int64) ([]Assignment, error) {
	const query = "SELECT id, class_id, subject_id, teacher_id FROM teaching_assignments WHERE class_id = ? ORDER BY id"

	rows, err := r.db.QueryContext(ctx, query, classID)
	if err != nil {
		return nil, fmt.Errorf("select assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var assignments []Assignment

	for rows.Next() {
		var assignment Assignment
		if err := rows.Scan(&assignment.ID, &assignment.ClassID, &assignment.SubjectID, &assignment.TeacherID); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}

		assignments = append(assignments, assignment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignments: %w", err)
	}

	return assignments, nil
}

func (r *AssignmentRepo) Upsert(ctx context.Context, classID, subjectID, teacherID int64) error {
	const query = `INSERT INTO teaching_assignments (class_id, subject_id, teacher_id) VALUES (?, ?, ?)
		ON CONFLICT (subject_id, class_id) DO UPDATE SET teacher_id = excluded.teacher_id`

	if _, err := r.db.ExecContext(ctx, query, classID, subjectID, teacherID); err != nil {
		return fmt.Errorf("upsert assignment: %w", err)
	}

	return nil
}

func (r *AssignmentRepo) Remove(ctx context.Context, id, classID int64) error {
	const query = "DELETE FROM teaching_assignments WHERE id = ? AND class_id = ?"

	result, err := r.db.ExecContext(ctx, query, id, classID)
	if err != nil {
		return fmt.Errorf("remove assignment: %w", err)
	}

	return requireAffected(result, "remove assignment")
}

func (r *AssignmentRepo) ListByTeacher(ctx context.Context, teacherID int64) ([]Assignment, error) {
	const query = "SELECT id, class_id, subject_id, teacher_id FROM teaching_assignments WHERE teacher_id = ? ORDER BY id"

	rows, err := r.db.QueryContext(ctx, query, teacherID)
	if err != nil {
		return nil, fmt.Errorf("select teacher assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var assignments []Assignment

	for rows.Next() {
		var assignment Assignment
		if err := rows.Scan(&assignment.ID, &assignment.ClassID, &assignment.SubjectID, &assignment.TeacherID); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}

		assignments = append(assignments, assignment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignments: %w", err)
	}

	return assignments, nil
}

func (r *AssignmentRepo) IsAssigned(ctx context.Context, classID, subjectID, teacherID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM teaching_assignments WHERE class_id = ? AND subject_id = ? AND teacher_id = ?)"

	var assigned bool
	if err := r.db.QueryRowContext(ctx, query, classID, subjectID, teacherID).Scan(&assigned); err != nil {
		return false, fmt.Errorf("check assignment: %w", err)
	}

	return assigned, nil
}
