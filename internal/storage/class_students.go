package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type ClassStudent struct {
	StudentID int64
	ClassID   int64
	ClassName string
}

type ClassStudentRepo struct {
	db *sql.DB
}

func NewClassStudentRepo(db *sql.DB) *ClassStudentRepo {
	return &ClassStudentRepo{db: db}
}

func (r *ClassStudentRepo) ListByYear(ctx context.Context, year int) ([]ClassStudent, error) {
	const query = `SELECT cs.student_id, c.id, c.name
		FROM class_students cs JOIN classes c ON c.id = cs.class_id
		WHERE c.year = ?`

	rows, err := r.db.QueryContext(ctx, query, year)
	if err != nil {
		return nil, fmt.Errorf("select class students: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var members []ClassStudent

	for rows.Next() {
		var member ClassStudent
		if err := rows.Scan(&member.StudentID, &member.ClassID, &member.ClassName); err != nil {
			return nil, fmt.Errorf("scan class student: %w", err)
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate class students: %w", err)
	}

	return members, nil
}

func (r *ClassStudentRepo) ClassOfStudent(ctx context.Context, studentID int64, year int) (Class, error) {
	const query = `SELECT c.id, c.year, c.name, c.active, c.created_at, c.updated_at
		FROM class_students cs JOIN classes c ON c.id = cs.class_id
		WHERE cs.student_id = ? AND c.year = ?`

	class, err := scanClass(r.db.QueryRowContext(ctx, query, studentID, year))
	if err != nil {
		return Class{}, fmt.Errorf("select class of student: %w", err)
	}

	return class, nil
}

func (r *ClassStudentRepo) SetForYear(ctx context.Context, studentID int64, year int, classID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const remove = `DELETE FROM class_students
		WHERE student_id = ? AND class_id IN (SELECT id FROM classes WHERE year = ?)`

	if _, err := tx.ExecContext(ctx, remove, studentID, year); err != nil {
		return fmt.Errorf("remove student from classes: %w", err)
	}

	if classID != 0 {
		const insert = "INSERT INTO class_students (class_id, student_id) VALUES (?, ?)"

		if _, err := tx.ExecContext(ctx, insert, classID, studentID); err != nil {
			return fmt.Errorf("add student to class: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
