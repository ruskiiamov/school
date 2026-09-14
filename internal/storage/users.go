package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type User struct {
	ID           int64
	Login        string
	PasswordHash string
	FullName     string
	Role         string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

const userColumns = "id, login, password_hash, full_name, role, active, created_at, updated_at"

func (r *UserRepo) ByLogin(ctx context.Context, login string) (User, error) {
	const query = "SELECT " + userColumns + " FROM users WHERE login = ?"

	user, err := scanUser(r.db.QueryRowContext(ctx, query, login))
	if err != nil {
		return User{}, fmt.Errorf("select user by login: %w", err)
	}

	return user, nil
}

func (r *UserRepo) Create(ctx context.Context, user User) (int64, error) {
	const query = `INSERT INTO users (login, password_hash, full_name, role, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query,
		user.Login, user.PasswordHash, user.FullName, user.Role, user.Active, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}

	return id, nil
}

func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id int64, hash string) error {
	const query = "UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?"

	if _, err := r.db.ExecContext(ctx, query, hash, toMillis(time.Now()), id); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	return nil
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (User, error) {
	const query = "SELECT " + userColumns + " FROM users WHERE id = ?"

	user, err := scanUser(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return User{}, fmt.Errorf("select user by id: %w", err)
	}

	return user, nil
}

func (r *UserRepo) ListByRole(ctx context.Context, role string, includeInactive bool) ([]User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE role = ?"
	if !includeInactive {
		query += " AND active = 1"
	}

	query += " ORDER BY full_name, id"

	rows, err := r.db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, fmt.Errorf("select users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User

	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (r *UserRepo) Update(ctx context.Context, id int64, login, fullName string) error {
	const query = "UPDATE users SET login = ?, full_name = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, login, fullName, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return requireAffected(result, "update user")
}

func (r *UserRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const query = "UPDATE users SET active = ?, updated_at = ? WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, active, toMillis(time.Now()), id)
	if err != nil {
		return fmt.Errorf("set user active: %w", err)
	}

	return requireAffected(result, "set user active")
}

func (r *UserRepo) CountActiveByRole(ctx context.Context, role string) (int, error) {
	const query = "SELECT COUNT(*) FROM users WHERE role = ? AND active = 1"

	var count int
	if err := r.db.QueryRowContext(ctx, query, role).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return count, nil
}

func (r *UserRepo) LoginExists(ctx context.Context, login string, excludeID int64) (bool, error) {
	const query = "SELECT EXISTS (SELECT 1 FROM users WHERE login = ? AND id != ?)"

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, login, excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check login: %w", err)
	}

	return exists, nil
}

func (r *UserRepo) LoginsWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	const query = "SELECT login FROM users WHERE substr(login, 1, ?) = ?"

	rows, err := r.db.QueryContext(ctx, query, len(prefix), prefix)
	if err != nil {
		return nil, fmt.Errorf("select logins by prefix: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var logins []string

	for rows.Next() {
		var login string
		if err := rows.Scan(&login); err != nil {
			return nil, fmt.Errorf("scan login: %w", err)
		}

		logins = append(logins, login)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate logins: %w", err)
	}

	return logins, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (User, error) {
	var (
		user      User
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.FullName, &user.Role, &user.Active,
		&createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	user.CreatedAt = fromMillis(createdAt)
	user.UpdatedAt = fromMillis(updatedAt)

	return user, nil
}
