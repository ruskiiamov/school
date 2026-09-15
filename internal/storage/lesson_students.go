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
