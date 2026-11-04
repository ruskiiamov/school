package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Penalty struct {
	ID          int64
	LessonID    int64
	StudentID   int64
	WorkTypeID  int64
	DonePercent int
	Points      int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type StudentPenalty struct {
	Penalty
	SubjectID int64
	Date      time.Time
}

type PenaltyChange struct {
	StudentID   int64
	WorkTypeID  int64
	DonePercent int
	Points      int
	Remove      bool
}

type PenaltyRepo struct {
	db *sql.DB
}

func NewPenaltyRepo(db *sql.DB) *PenaltyRepo {
	return &PenaltyRepo{db: db}
}

const penaltyColumns = "p.id, p.lesson_id, p.student_id, p.work_type_id, p.done_percent, p.points, p.created_at, p.updated_at"

func (r *PenaltyRepo) ListByLesson(ctx context.Context, lessonID int64) ([]Penalty, error) {
	const query = "SELECT " + penaltyColumns + " FROM penalties p WHERE p.lesson_id = ? ORDER BY p.id"

	rows, err := r.db.QueryContext(ctx, query, lessonID)
	if err != nil {
		return nil, fmt.Errorf("select lesson penalties: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return collectPenalties(rows)
}

func (r *PenaltyRepo) ListByPair(ctx context.Context, classID, subjectID int64) ([]Penalty, error) {
	const query = "SELECT " + penaltyColumns + ` FROM penalties p
		WHERE p.lesson_id IN (SELECT id FROM lessons WHERE class_id = ? AND subject_id = ?)
		ORDER BY p.id`

	rows, err := r.db.QueryContext(ctx, query, classID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("select pair penalties: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return collectPenalties(rows)
}

func (r *PenaltyRepo) ListByStudents(ctx context.Context, studentIDs []int64) ([]StudentPenalty, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	list, err := json.Marshal(studentIDs)
	if err != nil {
		return nil, fmt.Errorf("encode student ids: %w", err)
	}

	const query = "SELECT " + penaltyColumns + `, l.subject_id, l.date
		FROM penalties p JOIN lessons l ON l.id = p.lesson_id
		WHERE p.student_id IN (SELECT value FROM json_each(?))
		ORDER BY p.student_id, l.date, p.created_at, p.id`

	rows, err := r.db.QueryContext(ctx, query, string(list))
	if err != nil {
		return nil, fmt.Errorf("select student penalties: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var penalties []StudentPenalty

	for rows.Next() {
		var (
			penalty   StudentPenalty
			date      string
			createdAt int64
			updatedAt int64
		)

		err := rows.Scan(&penalty.ID, &penalty.LessonID, &penalty.StudentID, &penalty.WorkTypeID, &penalty.DonePercent,
			&penalty.Points, &createdAt, &updatedAt, &penalty.SubjectID, &date)
		if err != nil {
			return nil, fmt.Errorf("scan student penalty: %w", err)
		}

		if penalty.Date, err = fromDate(date); err != nil {
			return nil, fmt.Errorf("scan student penalty: %w", err)
		}

		penalty.CreatedAt = fromMillis(createdAt)
		penalty.UpdatedAt = fromMillis(updatedAt)
		penalties = append(penalties, penalty)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student penalties: %w", err)
	}

	return penalties, nil
}

func (r *PenaltyRepo) Totals(ctx context.Context) (map[int64]int, error) {
	const query = `SELECT p.student_id, SUM(p.points) FROM penalties p JOIN lessons l ON l.id = p.lesson_id
		GROUP BY p.student_id`

	return queryTotals(ctx, r.db, query, "penalty totals")
}

func (r *PenaltyRepo) Set(ctx context.Context, lessonID int64, changes []PenaltyChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const (
		upsert = `INSERT INTO penalties (lesson_id, student_id, work_type_id, done_percent, points, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (lesson_id, student_id, work_type_id) DO UPDATE SET
			done_percent = excluded.done_percent, points = excluded.points, updated_at = excluded.updated_at`
		remove = "DELETE FROM penalties WHERE lesson_id = ? AND student_id = ? AND work_type_id = ?"
	)

	now := toMillis(time.Now())

	for _, change := range changes {
		if change.Remove {
			if _, err := tx.ExecContext(ctx, remove, lessonID, change.StudentID, change.WorkTypeID); err != nil {
				return fmt.Errorf("delete penalty: %w", err)
			}

			continue
		}

		_, err := tx.ExecContext(ctx, upsert, lessonID, change.StudentID, change.WorkTypeID, change.DonePercent, change.Points, now, now)
		if err != nil {
			return fmt.Errorf("upsert penalty: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit penalties: %w", err)
	}

	return nil
}

func (r *PenaltyRepo) DeleteOrphaned(ctx context.Context) (int64, error) {
	const query = "DELETE FROM penalties WHERE lesson_id NOT IN (SELECT id FROM lessons)"

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned penalties: %w", err)
	}

	return rowsAffected(result)
}

func collectPenalties(rows *sql.Rows) ([]Penalty, error) {
	var penalties []Penalty

	for rows.Next() {
		var (
			penalty   Penalty
			createdAt int64
			updatedAt int64
		)

		err := rows.Scan(&penalty.ID, &penalty.LessonID, &penalty.StudentID, &penalty.WorkTypeID, &penalty.DonePercent,
			&penalty.Points, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan penalty: %w", err)
		}

		penalty.CreatedAt = fromMillis(createdAt)
		penalty.UpdatedAt = fromMillis(updatedAt)
		penalties = append(penalties, penalty)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate penalties: %w", err)
	}

	return penalties, nil
}

func queryTotals(ctx context.Context, db *sql.DB, query, operation string) (map[int64]int, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select %s: %w", operation, err)
	}
	defer func() { _ = rows.Close() }()

	totals := map[int64]int{}

	for rows.Next() {
		var (
			studentID int64
			total     int
		)

		if err := rows.Scan(&studentID, &total); err != nil {
			return nil, fmt.Errorf("scan %s: %w", operation, err)
		}

		totals[studentID] = total
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", operation, err)
	}

	return totals, nil
}

type Workoff struct {
	ID         int64
	StudentID  int64
	Date       time.Time
	Points     int
	Comment    string
	RecordedBy int64
	CreatedAt  time.Time
}

type WorkoffRepo struct {
	db *sql.DB
}

func NewWorkoffRepo(db *sql.DB) *WorkoffRepo {
	return &WorkoffRepo{db: db}
}

const workoffColumns = "id, student_id, date, points, comment, recorded_by, created_at"

func (r *WorkoffRepo) Create(ctx context.Context, workoff Workoff) (int64, error) {
	const query = `INSERT INTO penalty_workoffs (student_id, date, points, comment, recorded_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query, workoff.StudentID, toDate(workoff.Date), workoff.Points, workoff.Comment,
		workoff.RecordedBy, toMillis(time.Now()))
	if err != nil {
		return 0, fmt.Errorf("insert workoff: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert workoff: %w", err)
	}

	return id, nil
}

func (r *WorkoffRepo) ByID(ctx context.Context, id int64) (Workoff, error) {
	const query = "SELECT " + workoffColumns + " FROM penalty_workoffs WHERE id = ?"

	workoff, err := scanWorkoff(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Workoff{}, fmt.Errorf("select workoff by id: %w", err)
	}

	return workoff, nil
}

func (r *WorkoffRepo) ListByStudent(ctx context.Context, studentID int64) ([]Workoff, error) {
	const query = "SELECT " + workoffColumns + " FROM penalty_workoffs WHERE student_id = ? ORDER BY date DESC, id DESC"

	rows, err := r.db.QueryContext(ctx, query, studentID)
	if err != nil {
		return nil, fmt.Errorf("select workoffs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var workoffs []Workoff

	for rows.Next() {
		workoff, err := scanWorkoff(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workoff: %w", err)
		}

		workoffs = append(workoffs, workoff)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workoffs: %w", err)
	}

	return workoffs, nil
}

func (r *WorkoffRepo) Totals(ctx context.Context) (map[int64]int, error) {
	const query = "SELECT student_id, SUM(points) FROM penalty_workoffs GROUP BY student_id"

	return queryTotals(ctx, r.db, query, "workoff totals")
}

func (r *WorkoffRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM penalty_workoffs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete workoff: %w", err)
	}

	return requireAffected(result, "delete workoff")
}

func scanWorkoff(row scanner) (Workoff, error) {
	var (
		workoff   Workoff
		date      string
		createdAt int64
	)

	err := row.Scan(&workoff.ID, &workoff.StudentID, &date, &workoff.Points, &workoff.Comment, &workoff.RecordedBy, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Workoff{}, ErrNotFound
	}
	if err != nil {
		return Workoff{}, err
	}

	if workoff.Date, err = fromDate(date); err != nil {
		return Workoff{}, err
	}

	workoff.CreatedAt = fromMillis(createdAt)

	return workoff, nil
}
