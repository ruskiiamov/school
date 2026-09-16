package files_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

func TestDownloadByRole(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	ctx := t.Context()

	class, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)
	other, err := env.School.CreateClass(ctx, "9Б")
	require.NoError(t, err)
	subject, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	teacher := createUser(t, env, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	createUser(t, env, auth.RoleTeacher, "colleague", "Кузнецов Виктор Павлович")
	student := createUser(t, env, auth.RoleStudent, "student", "Иванова Мария Петровна")
	stranger := createUser(t, env, auth.RoleStudent, "stranger", "Чужой Ученик Иванович")
	parent := createUser(t, env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")
	createUser(t, env, auth.RoleParent, "childless", "Бездетный Родитель Иванович")

	require.NoError(t, env.School.AddClassStudent(ctx, class, student))
	require.NoError(t, env.School.AddClassStudent(ctx, other, stranger))
	require.NoError(t, env.School.AssignTeacher(ctx, class, subject, teacher))
	require.NoError(t, env.School.AddChild(ctx, parent, student))

	today := env.School.Today()
	lessonID, err := env.Journal.OpenLesson(ctx, teacher, env.School.CurrentYear(), today,
		journal.OpenLessonInput{ClassID: class, SubjectID: subject, Date: today.Format(validation.DateLayout)})
	require.NoError(t, err)
	require.NoError(t, env.Journal.AddHomeworkFile(ctx, teacher, lessonID, "задание.txt", strings.NewReader("Упр. 12")))

	homework, err := env.Journal.Homework(ctx, lessonID)
	require.NoError(t, err)
	require.Len(t, homework.Files, 1)
	path := "/files/" + homework.Files[0].ID

	tests := []struct {
		login string
		code  int
	}{
		{login: "teacher", code: http.StatusOK},
		{login: "student", code: http.StatusOK},
		{login: "parent", code: http.StatusOK},
		{login: "colleague", code: http.StatusNotFound},
		{login: "stranger", code: http.StatusNotFound},
		{login: "childless", code: http.StatusNotFound},
	}

	for _, tt := range tests {
		recorder := servertest.Get(t, env.Handler, path, env.LoginAs(t, tt.login))
		assert.Equal(t, tt.code, recorder.Code, tt.login)
	}

	admin := servertest.Get(t, env.Handler, path, servertest.Login(t, env.Handler))
	assert.Equal(t, http.StatusOK, admin.Code)
	assert.Equal(t, "Упр. 12", admin.Body.String())

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/files/nope", env.LoginAs(t, "teacher")).Code)

	anonymous := servertest.Get(t, env.Handler, path)
	servertest.AssertRedirect(t, anonymous, "/login")
}

func createUser(t *testing.T, env *servertest.Env, role auth.Role, login, fullName string) int64 {
	t.Helper()

	env.CreateUser(t, role, login, fullName)

	users, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: role, IncludeInactive: true})
	require.NoError(t, err)

	for _, user := range users {
		if user.Login == login {
			return user.ID
		}
	}

	t.Fatalf("user %q not found", login)

	return 0
}
