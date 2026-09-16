package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type LessonStudent struct {
	LessonID  int64
	StudentID int64
	Absent    bool
	Comment   string
	UpdatedAt time.Time
}

type LessonStudentRepo struct {
	db *sql.DB
}

func NewLessonStudentRepo(db *sql.DB) *LessonStudentRepo {
	return &LessonStudentRepo{db: db}
}

func (r *LessonStudentRepo) ListByLesson(ctx context.Context, lessonID int64) ([]LessonStudent, error) {
	const query = "SELECT lesson_id, student_id, absent, comment, updated_at FROM lesson_students WHERE lesson_id = ? ORDER BY student_id"

	rows, err := r.db.QueryContext(ctx, query, lessonID)
	if err != nil {
		return nil, fmt.Errorf("select lesson students: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []LessonStudent

	for rows.Next() {
		var (
			record    LessonStudent
			updatedAt int64
		)

		if err := rows.Scan(&record.LessonID, &record.StudentID, &record.Absent, &record.Comment, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan lesson student: %w", err)
		}

		record.UpdatedAt = fromMillis(updatedAt)
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lesson students: %w", err)
	}

	return records, nil
}

func (r *LessonStudentRepo) ListByPairPeriod(ctx context.Context, classID, subjectID int64, from, to time.Time) ([]LessonStudent, error) {
	const query = `SELECT lesson_id, student_id, absent, comment, updated_at FROM lesson_students
		WHERE lesson_id IN (SELECT id FROM lessons WHERE class_id = ? AND subject_id = ? AND date BETWEEN ? AND ?)
		ORDER BY lesson_id, student_id`

	rows, err := r.db.QueryContext(ctx, query, classID, subjectID, toDate(from), toDate(to))
	if err != nil {
		return nil, fmt.Errorf("select period lesson students: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []LessonStudent

	for rows.Next() {
		var (
			record    LessonStudent
			updatedAt int64
		)

		if err := rows.Scan(&record.LessonID, &record.StudentID, &record.Absent, &record.Comment, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan lesson student: %w", err)
		}

		record.UpdatedAt = fromMillis(updatedAt)
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lesson students: %w", err)
	}

	return records, nil
}

func (r *LessonStudentRepo) AbsencesByStudentPeriod(ctx context.Context, studentID int64, from, to time.Time) (map[int64]int, error) {
	const query = `SELECT l.subject_id, COUNT(*) FROM lesson_students ls JOIN lessons l ON l.id = ls.lesson_id
		WHERE ls.student_id = ? AND ls.absent = 1 AND l.date BETWEEN ? AND ?
		GROUP BY l.subject_id`

	rows, err := r.db.QueryContext(ctx, query, studentID, toDate(from), toDate(to))
	if err != nil {
		return nil, fmt.Errorf("count student absences: %w", err)
	}
	defer func() { _ = rows.Close() }()

	absences := map[int64]int{}

	for rows.Next() {
		var (
			subjectID int64
			count     int
		)

		if err := rows.Scan(&subjectID, &count); err != nil {
			return nil, fmt.Errorf("scan student absences: %w", err)
		}

		absences[subjectID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student absences: %w", err)
	}

	return absences, nil
}

func (r *LessonStudentRepo) Upsert(ctx context.Context, record LessonStudent) error {
	const query = `INSERT INTO lesson_students (lesson_id, student_id, absent, comment, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (lesson_id, student_id) DO UPDATE SET absent = excluded.absent, comment = excluded.comment, updated_at = excluded.updated_at`

	if _, err := r.db.ExecContext(ctx, query, record.LessonID, record.StudentID, record.Absent, record.Comment, toMillis(time.Now())); err != nil {
		return fmt.Errorf("upsert lesson student: %w", err)
	}

	return nil
}

func (r *LessonStudentRepo) Delete(ctx context.Context, lessonID, studentID int64) error {
	const query = "DELETE FROM lesson_students WHERE lesson_id = ? AND student_id = ?"

	if _, err := r.db.ExecContext(ctx, query, lessonID, studentID); err != nil {
		return fmt.Errorf("delete lesson student: %w", err)
	}

	return nil
}

func (r *LessonStudentRepo) DeleteOrphaned(ctx context.Context) (int64, error) {
	const query = "DELETE FROM lesson_students WHERE lesson_id NOT IN (SELECT id FROM lessons)"

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned lesson students: %w", err)
	}

	return rowsAffected(result)
}
