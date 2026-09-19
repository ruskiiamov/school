package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/validation"
)

func TestCreateUserGeneratesLoginAndPassword(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	first, err := svc.CreateUser(ctx, NewUser{Role: RoleStudent, Name: Name{Last: "Иванова", First: "Мария", Middle: "Петровна"}})
	require.NoError(t, err)
	assert.Equal(t, "ivanova.m", first.User.Login)
	assert.Equal(t, "Иванова Мария Петровна", first.User.FullName)
	assert.Equal(t, RoleStudent, first.User.Role)
	assert.True(t, first.User.Active)
	assert.Len(t, first.Password, generatedPasswordLength)
	assert.Regexp(t, "^[a-zA-Z0-9]+$", first.Password)

	second, err := svc.CreateUser(ctx, NewUser{Role: RoleParent, Name: Name{Last: "Иванова", First: "Марина"}})
	require.NoError(t, err)
	assert.Equal(t, "ivanova.m2", second.User.Login)

	third, err := svc.CreateUser(ctx, NewUser{Role: RoleTeacher, Name: Name{Last: "Иванова", First: "Мария"}})
	require.NoError(t, err)
	assert.Equal(t, "ivanova.m3", third.User.Login)
	assert.NotEqual(t, first.Password, third.Password)

	session, err := svc.Login(ctx, first.User.Login, first.Password)
	require.NoError(t, err)

	user, err := svc.Authenticate(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, first.User.ID, user.ID)
}

func TestCreateUserValidatesName(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()
	long := strings.Repeat("я", 51)

	tests := []struct {
		name  string
		input Name
		field string
		msg   string
	}{
		{"empty last name", Name{Last: "  ", First: "Анна"}, "last_name", msgLastNameRequired},
		{"empty first name", Name{Last: "Сидорова"}, "first_name", msgFirstNameRequired},
		{"long last name", Name{Last: long, First: "Анна"}, "last_name", msgNamePartTooLong},
		{"long middle name", Name{Last: "Сидорова", First: "Анна", Middle: long}, "middle_name", msgNamePartTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := svc.CreateUser(t.Context(), NewUser{Role: RoleTeacher, Name: tt.input})

			var errs validation.Errors
			require.ErrorAs(t, err, &errs)
			assert.Equal(t, tt.msg, errs[tt.field])
		})
	}

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleParent, Name: Name{Last: " Петров ", First: "Иван"}})
	require.NoError(t, err)
	assert.Equal(t, "Петров Иван", created.User.FullName)
	assert.Empty(t, created.User.Name.Middle)

	users, err := svc.Users(ctx, UserFilter{Role: RoleTeacher, IncludeInactive: true})
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUpdateUserValidatesLogin(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleTeacher, Name: Name{Last: "Сидорова", First: "Анна"}})
	require.NoError(t, err)

	other, err := svc.CreateUser(ctx, NewUser{Role: RoleParent, Name: Name{Last: "Петров", First: "Иван"}})
	require.NoError(t, err)

	tests := []struct {
		name  string
		input UserInput
		field string
		msg   string
	}{
		{"empty name", UserInput{Name: Name{}, Login: "anna"}, "last_name", msgLastNameRequired},
		{"empty login", UserInput{Name: Name{Last: "Сидорова", First: "Анна"}, Login: ""}, "login", msgLoginRequired},
		{"cyrillic login", UserInput{Name: Name{Last: "Сидорова", First: "Анна"}, Login: "анна"}, "login", msgLoginInvalid},
		{"login with space", UserInput{Name: Name{Last: "Сидорова", First: "Анна"}, Login: "an na"}, "login", msgLoginInvalid},
		{"taken login", UserInput{Name: Name{Last: "Сидорова", First: "Анна"}, Login: "PETROV.I"}, "login", msgLoginTaken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := svc.UpdateUser(t.Context(), created.User.ID, tt.input)

			var errs validation.Errors
			require.ErrorAs(t, err, &errs)
			assert.Equal(t, tt.msg, errs[tt.field])
		})
	}

	require.NoError(t, svc.UpdateUser(ctx, created.User.ID, UserInput{Name: Name{Last: "Сидорова", First: "Анна", Middle: "Андреевна"}, Login: "Anna"}))

	user, err := svc.UserByID(ctx, created.User.ID)
	require.NoError(t, err)
	assert.Equal(t, "anna", user.Login)
	assert.Equal(t, "Сидорова Анна Андреевна", user.FullName)

	untouched, err := svc.UserByID(ctx, other.User.ID)
	require.NoError(t, err)
	assert.Equal(t, "petrov.i", untouched.Login)

	assert.ErrorIs(t, svc.UpdateUser(ctx, 999, UserInput{Name: Name{Last: "X", First: "Y"}, Login: "x"}), ErrNotFound)

	_, err = svc.UserByID(ctx, 999)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestResetPasswordDropsSessions(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, time.Hour)
	ctx := t.Context()

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleTeacher, Name: Name{Last: "Сидорова", First: "Анна"}})
	require.NoError(t, err)

	session, err := svc.Login(ctx, created.User.Login, created.Password)
	require.NoError(t, err)

	generated, err := svc.ResetPassword(ctx, created.User.ID)
	require.NoError(t, err)
	assert.Len(t, generated, generatedPasswordLength)
	assert.NotEqual(t, created.Password, generated)
	assert.Equal(t, 0, countSessions(t, db))

	_, err = svc.Authenticate(ctx, session.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	_, err = svc.Login(ctx, created.User.Login, created.Password)
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.Login(ctx, created.User.Login, generated)
	assert.NoError(t, err)

	_, err = svc.ResetPassword(ctx, 999)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSetUserActiveDropsSessionsAndBlocksLogin(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, time.Hour)
	ctx := t.Context()

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleTeacher, Name: Name{Last: "Сидорова", First: "Анна"}})
	require.NoError(t, err)

	session, err := svc.Login(ctx, created.User.Login, created.Password)
	require.NoError(t, err)

	require.NoError(t, svc.SetUserActive(ctx, created.User.ID, false))
	assert.Equal(t, 0, countSessions(t, db))

	_, err = svc.Authenticate(ctx, session.ID)
	assert.ErrorIs(t, err, ErrUnauthenticated)

	_, err = svc.Login(ctx, created.User.Login, created.Password)
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	active, err := svc.Users(ctx, UserFilter{Role: RoleTeacher})
	require.NoError(t, err)
	assert.Empty(t, active)

	all, err := svc.Users(ctx, UserFilter{Role: RoleTeacher, IncludeInactive: true})
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.False(t, all[0].Active)

	require.NoError(t, svc.SetUserActive(ctx, created.User.ID, true))

	_, err = svc.Login(ctx, created.User.Login, created.Password)
	assert.NoError(t, err)

	assert.ErrorIs(t, svc.SetUserActive(ctx, 999, false), ErrNotFound)
}

func TestUsersFilterIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	for _, name := range []Name{{Last: "Иванова", First: "Мария"}, {Last: "Петров", First: "Иван"}, {Last: "Сидорова", First: "Анна"}} {
		_, err := svc.CreateUser(ctx, NewUser{Role: RoleStudent, Name: name})
		require.NoError(t, err)
	}

	_, err := svc.CreateUser(ctx, NewUser{Role: RoleTeacher, Name: Name{Last: "Иванов", First: "Пётр"}})
	require.NoError(t, err)

	found, err := svc.Users(ctx, UserFilter{Role: RoleStudent, Query: " иВаН "})
	require.NoError(t, err)
	require.Len(t, found, 2)
	assert.Equal(t, "Иванова Мария", found[0].FullName)
	assert.Equal(t, "Петров Иван", found[1].FullName)

	none, err := svc.Users(ctx, UserFilter{Role: RoleStudent, Query: "Кузнецов"})
	require.NoError(t, err)
	assert.Empty(t, none)
}
