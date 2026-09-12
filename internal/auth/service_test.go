package auth

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/storage"
)

func newTestService(t *testing.T, ttl time.Duration) (*Service, *sql.DB) {
	t.Helper()

	db, err := storage.Open(t.Context(), config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	require.NoError(t, storage.Migrate(db))

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewService(storage.NewUserRepo(db), storage.NewSessionRepo(db), ttl, log), db
}

func adminConfig() config.Admin {
	return config.Admin{Login: "admin", Password: "secret", FullName: "Администратор"}
}

func TestEnsureAdminCreatesUser(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))
	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	session, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	user, err := svc.Authenticate(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Login)
	assert.Equal(t, RoleAdmin, user.Role)
	assert.Equal(t, "Администратор", user.FullName)
}

func TestEnsureAdminUpdatesPasswordAndDropsSessions(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	session, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	changed := adminConfig()
	changed.Password = "new-secret"
	require.NoError(t, svc.EnsureAdmin(ctx, changed))

	_, err = svc.Authenticate(ctx, session.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	_, err = svc.Login(ctx, "admin", "secret")
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.Login(ctx, "admin", "new-secret")
	assert.NoError(t, err)
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()
	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	tests := []struct {
		name     string
		login    string
		password string
	}{
		{"wrong password", "admin", "wrong"},
		{"unknown login", "nobody", "secret"},
		{"empty login", "", ""},
		{"login is case sensitive", "Admin", "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := svc.Login(t.Context(), tt.login, tt.password)
			assert.ErrorIs(t, err, ErrInvalidCredentials)
		})
	}
}

func TestLoginIssuesDistinctSessions(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()
	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	first, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	second, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
	assert.WithinDuration(t, time.Now().Add(time.Hour), second.ExpiresAt, time.Minute)

	for _, id := range []string{first.ID, second.ID} {
		_, err := svc.Authenticate(ctx, id)
		assert.NoError(t, err)
	}
}

func TestAuthenticateRejectsExpiredSession(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, -time.Minute)
	ctx := t.Context()
	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	session, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	_, err = svc.Authenticate(ctx, session.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	assert.Equal(t, 0, countSessions(t, db))
}

func TestAuthenticateRejectsUnknownAndEmptySession(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)

	for _, id := range []string{"", "does-not-exist"} {
		_, err := svc.Authenticate(t.Context(), id)
		assert.ErrorIs(t, err, ErrUnauthenticated)
	}
}

func TestAuthenticateRejectsOrphanedSession(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, time.Hour)
	ctx := t.Context()
	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	session, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "DELETE FROM users")
	require.NoError(t, err)

	_, err = svc.Authenticate(ctx, session.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	assert.Equal(t, 1, countSessions(t, db))

	svc.cleanupSessions(ctx)
	assert.Equal(t, 0, countSessions(t, db))
}

func TestLogoutInvalidatesSession(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()
	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	session, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, session.ID))

	_, err = svc.Authenticate(ctx, session.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	assert.NoError(t, svc.Logout(ctx, session.ID))
}

func TestRunSessionCleanupStopsWithContext(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		svc.RunSessionCleanup(ctx, time.Hour)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup goroutine did not stop after context cancellation")
	}
}

func countSessions(t *testing.T, db *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM sessions").Scan(&count))

	return count
}
