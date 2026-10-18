package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Mark struct {
	ID         int64
	LessonID   int64
	StudentID  int64
	WorkTypeID int64
	Value      int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MarkRepo struct {
	db *sql.DB
}

func NewMarkRepo(db *sql.DB) *MarkRepo {
	return &MarkRepo{db: db}
}

const markColumns = "id, lesson_id, student_id, work_type_id, value, created_at, updated_at"

func (r *MarkRepo) ListByLesson(ctx context.Context, lessonID int64) ([]Mark, error) {
	const query = "SELECT " + markColumns + " FROM marks WHERE lesson_id = ? ORDER BY id"

	rows, err := r.db.QueryContext(ctx, query, lessonID)
	if err != nil {
		return nil, fmt.Errorf("select marks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var marks []Mark

	for rows.Next() {
		mark, err := scanMark(rows)
		if err != nil {
			return nil, fmt.Errorf("scan mark: %w", err)
		}

		marks = append(marks, mark)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate marks: %w", err)
	}

	return marks, nil
}

func (r *MarkRepo) ListByPair(ctx context.Context, classID, subjectID int64) ([]Mark, error) {
	const query = "SELECT " + markColumns + ` FROM marks
		WHERE lesson_id IN (SELECT id FROM lessons WHERE class_id = ? AND subject_id = ?)
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, classID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("select pair marks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return collectMarks(rows)
}

type StudentMark struct {
	Mark
	SubjectID int64
	Date      time.Time
}

func (r *MarkRepo) ListByStudentPeriod(ctx context.Context, studentID int64, from, to time.Time) ([]StudentMark, error) {
	const query = `SELECT m.id, m.lesson_id, m.student_id, m.work_type_id, m.value, m.created_at, m.updated_at, l.subject_id, l.date
		FROM marks m JOIN lessons l ON l.id = m.lesson_id
		WHERE m.student_id = ? AND l.date BETWEEN ? AND ?
		ORDER BY l.date, m.id`

	rows, err := r.db.QueryContext(ctx, query, studentID, toDate(from), toDate(to))
	if err != nil {
		return nil, fmt.Errorf("select student marks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var marks []StudentMark

	for rows.Next() {
		var (
			mark      StudentMark
			date      string
			createdAt int64
			updatedAt int64
		)

		err := rows.Scan(&mark.ID, &mark.LessonID, &mark.StudentID, &mark.WorkTypeID, &mark.Value,
			&createdAt, &updatedAt, &mark.SubjectID, &date)
		if err != nil {
			return nil, fmt.Errorf("scan student mark: %w", err)
		}

		if mark.Date, err = fromDate(date); err != nil {
			return nil, fmt.Errorf("scan student mark: %w", err)
		}

		mark.CreatedAt = fromMillis(createdAt)
		mark.UpdatedAt = fromMillis(updatedAt)
		marks = append(marks, mark)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student marks: %w", err)
	}

	return marks, nil
}

func collectMarks(rows *sql.Rows) ([]Mark, error) {
	var marks []Mark

	for rows.Next() {
		mark, err := scanMark(rows)
		if err != nil {
			return nil, fmt.Errorf("scan mark: %w", err)
		}

		marks = append(marks, mark)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate marks: %w", err)
	}

	return marks, nil
}

type MarkChange struct {
	WorkTypeID int64
	Value      int
}

func (r *MarkRepo) SetForStudent(ctx context.Context, lessonID, studentID int64, changes []MarkChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const (
		upsert = `INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (lesson_id, student_id, work_type_id) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
		remove = "DELETE FROM marks WHERE lesson_id = ? AND student_id = ? AND work_type_id = ?"
	)

	now := toMillis(time.Now())

	for _, change := range changes {
		if change.Value == 0 {
			if _, err := tx.ExecContext(ctx, remove, lessonID, studentID, change.WorkTypeID); err != nil {
				return fmt.Errorf("delete student mark: %w", err)
			}

			continue
		}

		if _, err := tx.ExecContext(ctx, upsert, lessonID, studentID, change.WorkTypeID, change.Value, now, now); err != nil {
			return fmt.Errorf("upsert student mark: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit student marks: %w", err)
	}

	return nil
}

func (r *MarkRepo) DeleteOrphaned(ctx context.Context) (int64, error) {
	const query = "DELETE FROM marks WHERE lesson_id NOT IN (SELECT id FROM lessons)"

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned marks: %w", err)
	}

	return rowsAffected(result)
}

func scanMark(row scanner) (Mark, error) {
	var (
		mark      Mark
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&mark.ID, &mark.LessonID, &mark.StudentID, &mark.WorkTypeID, &mark.Value, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Mark{}, ErrNotFound
	}
	if err != nil {
		return Mark{}, err
	}

	mark.CreatedAt = fromMillis(createdAt)
	mark.UpdatedAt = fromMillis(updatedAt)

	return mark, nil
}
