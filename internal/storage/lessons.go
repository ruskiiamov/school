package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Lesson struct {
	ID        int64
	ClassID   int64
	SubjectID int64
	TeacherID int64
	Date      time.Time
	Topic     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LessonRepo struct {
	db *sql.DB
}

func NewLessonRepo(db *sql.DB) *LessonRepo {
	return &LessonRepo{db: db}
}

const lessonColumns = "id, class_id, subject_id, teacher_id, date, topic, created_at, updated_at"

func (r *LessonRepo) ByID(ctx context.Context, id int64) (Lesson, error) {
	const query = "SELECT " + lessonColumns + " FROM lessons WHERE id = ?"

	lesson, err := scanLesson(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Lesson{}, fmt.Errorf("select lesson by id: %w", err)
	}

	return lesson, nil
}

func (r *LessonRepo) Find(ctx context.Context, classID, subjectID int64, date time.Time) (Lesson, error) {
	const query = "SELECT " + lessonColumns + " FROM lessons WHERE class_id = ? AND subject_id = ? AND date = ?"

	lesson, err := scanLesson(r.db.QueryRowContext(ctx, query, classID, subjectID, toDate(date)))
	if err != nil {
		return Lesson{}, fmt.Errorf("find lesson: %w", err)
	}

	return lesson, nil
}

const visibleLessons = ` FROM lessons
		WHERE class_id IN (SELECT id FROM classes WHERE year = ?)
			AND (teacher_id = ? OR EXISTS (
				SELECT 1 FROM teaching_assignments a
				WHERE a.class_id = lessons.class_id AND a.subject_id = lessons.subject_id AND a.teacher_id = ?))`

func (r *LessonRepo) CountVisible(ctx context.Context, teacherID int64, year int) (int, error) {
	const query = "SELECT COUNT(*)" + visibleLessons

	var count int
	if err := r.db.QueryRowContext(ctx, query, year, teacherID, teacherID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count lessons: %w", err)
	}

	return count, nil
}

func (r *LessonRepo) ListVisible(ctx context.Context, teacherID int64, year, limit, offset int) ([]Lesson, error) {
	const query = "SELECT " + lessonColumns + visibleLessons + `
		ORDER BY date DESC, id DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, year, teacherID, teacherID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("select lessons: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var lessons []Lesson

	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}

		lessons = append(lessons, lesson)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lessons: %w", err)
	}

	return lessons, nil
}

func (r *LessonRepo) ListByPair(ctx context.Context, classID, subjectID int64) ([]Lesson, error) {
	const query = "SELECT " + lessonColumns + " FROM lessons WHERE class_id = ? AND subject_id = ? ORDER BY date DESC, id DESC"

	rows, err := r.db.QueryContext(ctx, query, classID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("select pair lessons: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var lessons []Lesson

	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}

		lessons = append(lessons, lesson)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lessons: %w", err)
	}

	return lessons, nil
}

func (r *LessonRepo) ListByPairPeriod(ctx context.Context, classID, subjectID int64, from, to time.Time) ([]Lesson, error) {
	const query = "SELECT " + lessonColumns + " FROM lessons WHERE class_id = ? AND subject_id = ? AND date BETWEEN ? AND ? ORDER BY date, id"

	rows, err := r.db.QueryContext(ctx, query, classID, subjectID, toDate(from), toDate(to))
	if err != nil {
		return nil, fmt.Errorf("select period lessons: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var lessons []Lesson

	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}

		lessons = append(lessons, lesson)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lessons: %w", err)
	}

	return lessons, nil
}

func (r *LessonRepo) Create(ctx context.Context, lesson Lesson) (int64, error) {
	const query = `INSERT INTO lessons (class_id, subject_id, teacher_id, date, topic, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query,
		lesson.ClassID, lesson.SubjectID, lesson.TeacherID, toDate(lesson.Date), lesson.Topic, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert lesson: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert lesson: %w", err)
	}

	return id, nil
}

func (r *LessonRepo) UpdateTopic(ctx context.Context, id int64, topic string) error {
	const query = "UPDATE lessons SET topic = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, topic, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("update lesson topic: %w", err)
	}

	return requireAffected(result, "update lesson topic")
}

func (r *LessonRepo) HasRecords(ctx context.Context, id int64) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM marks WHERE lesson_id = ?)
		OR EXISTS (SELECT 1 FROM lesson_students WHERE lesson_id = ?)`

	var has bool
	if err := r.db.QueryRowContext(ctx, query, id, id).Scan(&has); err != nil {
		return false, fmt.Errorf("check lesson records: %w", err)
	}

	return has, nil
}

func (r *LessonRepo) DeleteEmpty(ctx context.Context, id int64) error {
	const query = `DELETE FROM lessons WHERE id = ?
		AND NOT EXISTS (SELECT 1 FROM marks WHERE lesson_id = ?)
		AND NOT EXISTS (SELECT 1 FROM lesson_students WHERE lesson_id = ?)`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete lesson: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, query, id, id, id)
	if err != nil {
		return fmt.Errorf("delete lesson: %w", err)
	}

	if err := requireAffected(result, "delete lesson"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM homework WHERE lesson_id = ?", id); err != nil {
		return fmt.Errorf("delete lesson homework: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM homework_files WHERE lesson_id = ?", id); err != nil {
		return fmt.Errorf("delete lesson homework files: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete lesson: %w", err)
	}

	return nil
}

func scanLesson(row scanner) (Lesson, error) {
	var (
		lesson    Lesson
		date      string
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&lesson.ID, &lesson.ClassID, &lesson.SubjectID, &lesson.TeacherID, &date, &lesson.Topic, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Lesson{}, ErrNotFound
	}
	if err != nil {
		return Lesson{}, err
	}

	if lesson.Date, err = fromDate(date); err != nil {
		return Lesson{}, err
	}

	lesson.CreatedAt = fromMillis(createdAt)
	lesson.UpdatedAt = fromMillis(updatedAt)

	return lesson, nil
}

func (r *LessonRepo) ListForStudent(ctx context.Context, studentID int64, date time.Time) ([]Lesson, error) {
	const query = "SELECT " + lessonColumns + ` FROM lessons
		WHERE date = ? AND (
			class_id IN (SELECT class_id FROM class_students WHERE student_id = ?)
			OR id IN (SELECT lesson_id FROM marks WHERE student_id = ?)
			OR id IN (SELECT lesson_id FROM lesson_students WHERE student_id = ?))
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, toDate(date), studentID, studentID, studentID)
	if err != nil {
		return nil, fmt.Errorf("select student lessons: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var lessons []Lesson

	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}

		lessons = append(lessons, lesson)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lessons: %w", err)
	}

	return lessons, nil
}

func (r *LessonRepo) VisibleToStudent(ctx context.Context, studentID, lessonID int64) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM lessons WHERE id = ? AND (
		class_id IN (SELECT class_id FROM class_students WHERE student_id = ?)
		OR id IN (SELECT lesson_id FROM marks WHERE student_id = ?)
		OR id IN (SELECT lesson_id FROM lesson_students WHERE student_id = ?)))`

	var visible bool
	if err := r.db.QueryRowContext(ctx, query, lessonID, studentID, studentID, studentID).Scan(&visible); err != nil {
		return false, fmt.Errorf("check lesson visibility: %w", err)
	}

	return visible, nil
}
