package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type HomeworkFile struct {
	ID          string
	LessonID    int64
	Name        string
	Size        int64
	ContentType string
	CreatedAt   time.Time
}

type HomeworkFileRepo struct {
	db *sql.DB
}

func NewHomeworkFileRepo(db *sql.DB) *HomeworkFileRepo {
	return &HomeworkFileRepo{db: db}
}

const homeworkFileColumns = "id, lesson_id, name, size, content_type, created_at"

func (r *HomeworkFileRepo) ByID(ctx context.Context, id string) (HomeworkFile, error) {
	const query = "SELECT " + homeworkFileColumns + " FROM homework_files WHERE id = ?"

	file, err := scanHomeworkFile(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return HomeworkFile{}, fmt.Errorf("select homework file: %w", err)
	}

	return file, nil
}

func (r *HomeworkFileRepo) ListByLesson(ctx context.Context, lessonID int64) ([]HomeworkFile, error) {
	const query = "SELECT " + homeworkFileColumns + " FROM homework_files WHERE lesson_id = ? ORDER BY created_at, id"

	return r.list(ctx, query, lessonID)
}

func (r *HomeworkFileRepo) List(ctx context.Context) ([]HomeworkFile, error) {
	const query = "SELECT " + homeworkFileColumns + " FROM homework_files ORDER BY id"

	return r.list(ctx, query)
}

func (r *HomeworkFileRepo) list(ctx context.Context, query string, args ...any) ([]HomeworkFile, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select homework files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var files []HomeworkFile

	for rows.Next() {
		file, err := scanHomeworkFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan homework file: %w", err)
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate homework files: %w", err)
	}

	return files, nil
}

func (r *HomeworkFileRepo) CountByLesson(ctx context.Context, lessonID int64) (int, error) {
	const query = "SELECT COUNT(*) FROM homework_files WHERE lesson_id = ?"

	var count int
	if err := r.db.QueryRowContext(ctx, query, lessonID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count homework files: %w", err)
	}

	return count, nil
}

func (r *HomeworkFileRepo) Create(ctx context.Context, file HomeworkFile) error {
	const query = "INSERT INTO homework_files (id, lesson_id, name, size, content_type, created_at) VALUES (?, ?, ?, ?, ?, ?)"

	if _, err := r.db.ExecContext(ctx, query, file.ID, file.LessonID, file.Name, file.Size, file.ContentType, toMillis(time.Now())); err != nil {
		return fmt.Errorf("insert homework file: %w", err)
	}

	return nil
}

func (r *HomeworkFileRepo) Delete(ctx context.Context, id string) error {
	const query = "DELETE FROM homework_files WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete homework file: %w", err)
	}

	return requireAffected(result, "delete homework file")
}

func (r *HomeworkFileRepo) DeleteOrphaned(ctx context.Context) ([]string, error) {
	const query = "DELETE FROM homework_files WHERE lesson_id NOT IN (SELECT id FROM lessons) RETURNING id"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("delete orphaned homework files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan orphaned homework file: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orphaned homework files: %w", err)
	}

	return ids, nil
}

func scanHomeworkFile(row scanner) (HomeworkFile, error) {
	var (
		file      HomeworkFile
		createdAt int64
	)

	err := row.Scan(&file.ID, &file.LessonID, &file.Name, &file.Size, &file.ContentType, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return HomeworkFile{}, ErrNotFound
	}
	if err != nil {
		return HomeworkFile{}, err
	}

	file.CreatedAt = fromMillis(createdAt)

	return file, nil
}
