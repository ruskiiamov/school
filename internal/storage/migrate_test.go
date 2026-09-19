package storage

import (
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/storage/migrations"
)

func TestUserNamesMigrationSplitsFullName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := Open(ctx, config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	require.NoError(t, err)

	_, err = provider.UpTo(ctx, 4)
	require.NoError(t, err)

	for login, fullName := range map[string]string{
		"three":  "Сидорова Анна Андреевна",
		"two":    "Петров Иван",
		"one":    "Администратор",
		"spaces": "  Козлова   Мария  Ильинична оглы ",
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (login, password_hash, full_name, role, active, created_at, updated_at)
			VALUES (?, 'hash', ?, 'teacher', 1, 0, 0)`, login, fullName)
		require.NoError(t, err)
	}

	_, err = provider.Up(ctx)
	require.NoError(t, err)

	repo := NewUserRepo(db)

	tests := []struct {
		login  string
		last   string
		first  string
		middle string
	}{
		{"three", "Сидорова", "Анна", "Андреевна"},
		{"two", "Петров", "Иван", ""},
		{"one", "Администратор", "", ""},
		{"spaces", "Козлова", "Мария", "Ильинична оглы"},
	}

	for _, tt := range tests {
		user, err := repo.ByLogin(ctx, tt.login)
		require.NoError(t, err, tt.login)
		assert.Equal(t, []string{tt.last, tt.first, tt.middle}, []string{user.LastName, user.FirstName, user.MiddleName}, tt.login)
	}
}
