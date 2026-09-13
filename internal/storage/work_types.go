package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type WorkType struct {
	ID        int64
	Name      string
	SortOrder int
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WorkTypeRepo struct {
	db *sql.DB
}

func NewWorkTypeRepo(db *sql.DB) *WorkTypeRepo {
	return &WorkTypeRepo{db: db}
}

const (
	workTypeColumns = "id, name, sort_order, active, created_at, updated_at"
	sortOrderStep   = 10
)

func (r *WorkTypeRepo) List(ctx context.Context, includeInactive bool) ([]WorkType, error) {
	query := "SELECT " + workTypeColumns + " FROM work_types"
	if !includeInactive {
		query += " WHERE active = 1"
	}

	query += " ORDER BY sort_order, id"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select work types: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var workTypes []WorkType

	for rows.Next() {
		workType, err := scanWorkType(rows)
		if err != nil {
			return nil, fmt.Errorf("scan work type: %w", err)
		}

		workTypes = append(workTypes, workType)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work types: %w", err)
	}

	return workTypes, nil
}

func (r *WorkTypeRepo) ByID(ctx context.Context, id int64) (WorkType, error) {
	const query = "SELECT " + workTypeColumns + " FROM work_types WHERE id = ?"

	workType, err := scanWorkType(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return WorkType{}, fmt.Errorf("select work type by id: %w", err)
	}

	return workType, nil
}

func (r *WorkTypeRepo) Create(ctx context.Context, name string, sortOrder int) (int64, error) {
	const query = `INSERT INTO work_types (name, sort_order, active, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?)`

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query, name, sortOrder, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert work type: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert work type: %w", err)
	}

	return id, nil
}

func (r *WorkTypeRepo) Update(ctx context.Context, id int64, name string) error {
	const query = "UPDATE work_types SET name = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, name, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("update work type: %w", err)
	}

	return requireAffected(result, "update work type")
}

func (r *WorkTypeRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const query = "UPDATE work_types SET active = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, active, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("set work type active: %w", err)
	}

	return requireAffected(result, "set work type active")
}

func (r *WorkTypeRepo) ActiveNameExists(ctx context.Context, name string, excludeID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM work_types WHERE active = 1 AND name = ? AND id != ?)"

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, name, excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check work type name: %w", err)
	}

	return exists, nil
}

func (r *WorkTypeRepo) NextSortOrder(ctx context.Context) (int, error) {
	const query = "SELECT COALESCE(MAX(sort_order), 0) FROM work_types"

	var highest int
	if err := r.db.QueryRowContext(ctx, query).Scan(&highest); err != nil {
		return 0, fmt.Errorf("select max work type order: %w", err)
	}

	return highest + sortOrderStep, nil
}

func (r *WorkTypeRepo) Reorder(ctx context.Context, ids []int64) error {
	const query = "UPDATE work_types SET sort_order = ?, updated_at = ? WHERE id = ?"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := toMillis(time.Now())

	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, query, (i+1)*sortOrderStep, now, id); err != nil {
			return fmt.Errorf("reorder work type %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}

	return nil
}

func scanWorkType(row scanner) (WorkType, error) {
	var (
		workType  WorkType
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&workType.ID, &workType.Name, &workType.SortOrder, &workType.Active, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkType{}, ErrNotFound
	}
	if err != nil {
		return WorkType{}, err
	}

	workType.CreatedAt = fromMillis(createdAt)
	workType.UpdatedAt = fromMillis(updatedAt)

	return workType, nil
}
