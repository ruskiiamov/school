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
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

const userColumns = "id, login, password_hash, full_name, role, created_at, updated_at"

func (r *UserRepo) ByLogin(ctx context.Context, login string) (User, error) {
	const query = "SELECT " + userColumns + " FROM users WHERE login = ?"

	user, err := scanUser(r.db.QueryRowContext(ctx, query, login))
	if err != nil {
		return User{}, fmt.Errorf("select user by login: %w", err)
	}

	return user, nil
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (User, error) {
	const query = "SELECT " + userColumns + " FROM users WHERE id = ?"

	user, err := scanUser(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return User{}, fmt.Errorf("select user by id: %w", err)
	}

	return user, nil
}

func (r *UserRepo) Create(ctx context.Context, user User) (int64, error) {
	const query = `INSERT INTO users (login, password_hash, full_name, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	now := toMillis(time.Now())

	result, err := r.db.ExecContext(ctx, query, user.Login, user.PasswordHash, user.FullName, user.Role, now, now)
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

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (User, error) {
	var (
		user      User
		createdAt int64
		updatedAt int64
	)

	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.FullName, &user.Role, &createdAt, &updatedAt)
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
