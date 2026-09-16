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

func (r *ClassStudentRepo) CountActiveByYear(ctx context.Context, year int) (int, error) {
	const query = `SELECT COUNT(DISTINCT cs.student_id)
		FROM class_students cs
		JOIN classes c ON c.id = cs.class_id
		JOIN users u ON u.id = cs.student_id
		WHERE c.year = ? AND c.active = 1 AND u.active = 1`

	var count int
	if err := r.db.QueryRowContext(ctx, query, year).Scan(&count); err != nil {
		return 0, fmt.Errorf("count class students: %w", err)
	}

	return count, nil
}

func (r *ClassStudentRepo) CountByClass(ctx context.Context, classID int64) (int, error) {
	const query = "SELECT COUNT(*) FROM class_students WHERE class_id = ?"

	var count int
	if err := r.db.QueryRowContext(ctx, query, classID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count class members: %w", err)
	}

	return count, nil
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

func (r *ClassStudentRepo) Add(ctx context.Context, classID, studentID int64) error {
	const query = "INSERT INTO class_students (class_id, student_id) VALUES (?, ?)"

	if _, err := r.db.ExecContext(ctx, query, classID, studentID); err != nil {
		return fmt.Errorf("add student to class: %w", err)
	}

	return nil
}

func (r *ClassStudentRepo) Remove(ctx context.Context, classID, studentID int64) error {
	const query = "DELETE FROM class_students WHERE class_id = ? AND student_id = ?"

	result, err := r.db.ExecContext(ctx, query, classID, studentID)
	if err != nil {
		return fmt.Errorf("remove student from class: %w", err)
	}

	return requireAffected(result, "remove student from class")
}

func (r *ClassStudentRepo) StudentIDs(ctx context.Context, classID int64) ([]int64, error) {
	const query = "SELECT student_id FROM class_students WHERE class_id = ? ORDER BY student_id"

	rows, err := r.db.QueryContext(ctx, query, classID)
	if err != nil {
		return nil, fmt.Errorf("select class student ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []int64

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan class student id: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate class student ids: %w", err)
	}

	return ids, nil
}

type Member struct {
	StudentID int64
	Active    bool
}

func (r *ClassStudentRepo) Members(ctx context.Context, classID int64) ([]Member, error) {
	const query = `SELECT cs.student_id, u.active
		FROM class_students cs JOIN users u ON u.id = cs.student_id
		WHERE cs.class_id = ? ORDER BY cs.student_id`

	rows, err := r.db.QueryContext(ctx, query, classID)
	if err != nil {
		return nil, fmt.Errorf("select class members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var members []Member

	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.StudentID, &member.Active); err != nil {
			return nil, fmt.Errorf("scan class member: %w", err)
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate class members: %w", err)
	}

	return members, nil
}

func (r *ClassStudentRepo) IsMember(ctx context.Context, classID, studentID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM class_students WHERE class_id = ? AND student_id = ?)"

	var member bool
	if err := r.db.QueryRowContext(ctx, query, classID, studentID).Scan(&member); err != nil {
		return false, fmt.Errorf("check class membership: %w", err)
	}

	return member, nil
}
