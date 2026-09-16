package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Homework struct {
	LessonID  int64
	Text      string
	DueDate   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type HomeworkRepo struct {
	db *sql.DB
}

func NewHomeworkRepo(db *sql.DB) *HomeworkRepo {
	return &HomeworkRepo{db: db}
}

const homeworkColumns = "lesson_id, text, due_date, created_at, updated_at"

func (r *HomeworkRepo) ByLesson(ctx context.Context, lessonID int64) (Homework, error) {
	const query = "SELECT " + homeworkColumns + " FROM homework WHERE lesson_id = ?"

	homework, err := scanHomework(r.db.QueryRowContext(ctx, query, lessonID))
	if err != nil {
		return Homework{}, fmt.Errorf("select homework: %w", err)
	}

	return homework, nil
}

func (r *HomeworkRepo) ByLessons(ctx context.Context, lessonIDs []int64) (map[int64]Homework, error) {
	result := make(map[int64]Homework, len(lessonIDs))

	for _, lessonID := range lessonIDs {
		homework, err := r.ByLesson(ctx, lessonID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}

		result[lessonID] = homework
	}

	return result, nil
}

func (r *HomeworkRepo) LessonIDsByPair(ctx context.Context, classID, subjectID int64) ([]int64, error) {
	const query = `SELECT lesson_id FROM homework
		WHERE lesson_id IN (SELECT id FROM lessons WHERE class_id = ? AND subject_id = ?)
		UNION SELECT lesson_id FROM homework_files
		WHERE lesson_id IN (SELECT id FROM lessons WHERE class_id = ? AND subject_id = ?)`

	rows, err := r.db.QueryContext(ctx, query, classID, subjectID, classID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("select lessons with homework: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []int64

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan lesson id: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lessons with homework: %w", err)
	}

	return ids, nil
}

func (r *HomeworkRepo) Upsert(ctx context.Context, homework Homework) error {
	const query = `INSERT INTO homework (lesson_id, text, due_date, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (lesson_id) DO UPDATE SET text = excluded.text, due_date = excluded.due_date, updated_at = excluded.updated_at`

	now := toMillis(time.Now())

	if _, err := r.db.ExecContext(ctx, query, homework.LessonID, homework.Text, toNullDate(homework.DueDate), now, now); err != nil {
		return fmt.Errorf("upsert homework: %w", err)
	}

	return nil
}

func (r *HomeworkRepo) Delete(ctx context.Context, lessonID int64) error {
	const query = "DELETE FROM homework WHERE lesson_id = ?"

	if _, err := r.db.ExecContext(ctx, query, lessonID); err != nil {
		return fmt.Errorf("delete homework: %w", err)
	}

	return nil
}

func (r *HomeworkRepo) DeleteOrphaned(ctx context.Context) (int64, error) {
	const query = "DELETE FROM homework WHERE lesson_id NOT IN (SELECT id FROM lessons)"

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned homework: %w", err)
	}

	return rowsAffected(result)
}

func scanHomework(row scanner) (Homework, error) {
	var (
		homework  Homework
		dueDate   sql.NullString
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&homework.LessonID, &homework.Text, &dueDate, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Homework{}, ErrNotFound
	}
	if err != nil {
		return Homework{}, err
	}

	if homework.DueDate, err = fromNullDate(dueDate); err != nil {
		return Homework{}, err
	}

	homework.CreatedAt = fromMillis(createdAt)
	homework.UpdatedAt = fromMillis(updatedAt)

	return homework, nil
}
