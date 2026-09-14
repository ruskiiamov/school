package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/validation"
)

func TestChangePasswordKeepsCurrentSessionOnly(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleTeacher, FullName: "Сидорова Анна Андреевна"})
	require.NoError(t, err)

	current, err := svc.Login(ctx, created.User.Login, created.Password)
	require.NoError(t, err)

	other, err := svc.Login(ctx, created.User.Login, created.Password)
	require.NoError(t, err)

	err = svc.ChangePassword(ctx, created.User.ID, current.ID, PasswordChange{
		Current: created.Password, New: "новый-пароль", Repeat: "новый-пароль",
	})
	require.NoError(t, err)

	_, err = svc.Authenticate(ctx, current.ID)
	assert.NoError(t, err)

	_, err = svc.Authenticate(ctx, other.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	_, err = svc.Login(ctx, created.User.Login, created.Password)
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.Login(ctx, created.User.Login, "новый-пароль")
	assert.NoError(t, err)
}

func TestChangePasswordValidatesInput(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleStudent, FullName: "Козлов Пётр Ильич"})
	require.NoError(t, err)

	session, err := svc.Login(ctx, created.User.Login, created.Password)
	require.NoError(t, err)

	tests := []struct {
		name  string
		input PasswordChange
		want  validation.Errors
	}{
		{
			name:  "empty",
			input: PasswordChange{},
			want:  validation.Errors{"current": msgCurrentRequired, "new": msgNewRequired},
		},
		{
			name:  "wrong current",
			input: PasswordChange{Current: "wrong", New: "long-enough", Repeat: "long-enough"},
			want:  validation.Errors{"current": msgCurrentWrong},
		},
		{
			name:  "short new",
			input: PasswordChange{Current: created.Password, New: "short", Repeat: "short"},
			want:  validation.Errors{"new": msgNewTooShort},
		},
		{
			name:  "too long new",
			input: PasswordChange{Current: created.Password, New: strings.Repeat("я", 40), Repeat: strings.Repeat("я", 40)},
			want:  validation.Errors{"new": msgNewTooLong},
		},
		{
			name:  "mismatch",
			input: PasswordChange{Current: created.Password, New: "long-enough", Repeat: "long-enough2"},
			want:  validation.Errors{"repeat": msgRepeatMismatch},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ChangePassword(ctx, created.User.ID, session.ID, tt.input)

			var errs validation.Errors
			require.ErrorAs(t, err, &errs)
			assert.Equal(t, tt.want, errs)
		})
	}

	_, err = svc.Authenticate(ctx, session.ID)
	assert.NoError(t, err)

	_, err = svc.Login(ctx, created.User.Login, created.Password)
	assert.NoError(t, err)
}

func TestChangePasswordUnknownUser(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)

	err := svc.ChangePassword(t.Context(), 999, "", PasswordChange{Current: "x", New: "long-enough", Repeat: "long-enough"})
	assert.ErrorIs(t, err, ErrNotFound)
}
