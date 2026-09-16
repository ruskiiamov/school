package diary_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

type fixture struct {
	env      *servertest.Env
	today    time.Time
	class    int64
	other    int64
	algebra  int64
	history  int64
	teacher  int64
	student  int64
	stranger int64
	workType int64
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	env := servertest.New(t)
	ctx := t.Context()

	class, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)
	other, err := env.School.CreateClass(ctx, "9Б")
	require.NoError(t, err)
	algebra, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)
	history, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "История"})
	require.NoError(t, err)

	teacher := createUser(t, env, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	student := createUser(t, env, auth.RoleStudent, "student", "Иванова Мария Петровна")
	stranger := createUser(t, env, auth.RoleStudent, "stranger", "Чужой Ученик Иванович")

	require.NoError(t, env.School.AddClassStudent(ctx, class, student))
	require.NoError(t, env.School.AddClassStudent(ctx, other, stranger))
	require.NoError(t, env.School.AssignTeacher(ctx, class, algebra, teacher))
	require.NoError(t, env.School.AssignTeacher(ctx, class, history, teacher))
	require.NoError(t, env.School.AssignTeacher(ctx, other, algebra, teacher))

	workTypes, err := env.School.WorkTypes(ctx, false)
	require.NoError(t, err)

	return fixture{
		env: env, today: env.School.Today(), class: class, other: other, algebra: algebra, history: history,
		teacher: teacher, student: student, stranger: stranger, workType: workTypes[0].ID,
	}
}

func (f fixture) year() string {
	return strconv.Itoa(f.env.School.CurrentYear())
}

func (f fixture) openLesson(t *testing.T, classID, subjectID int64, date time.Time) int64 {
	t.Helper()

	id, err := f.env.Journal.OpenLesson(t.Context(), f.teacher, f.env.School.CurrentYear(), f.today,
		journal.OpenLessonInput{ClassID: classID, SubjectID: subjectID, Date: date.Format(validation.DateLayout)})
	require.NoError(t, err)

	return id
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

func diaryURL(path string, date time.Time, params url.Values) string {
	query := url.Values{"date": {date.Format(validation.DateLayout)}}
	for name, values := range params {
		query[name] = values
	}

	return path + "?" + query.Encode()
}

func idValue(id int64) string {
	return strconv.FormatInt(id, 10)
}

func get(t *testing.T, env *servertest.Env, path string, cookie *http.Cookie, htmx bool) (int, string) {
	t.Helper()

	options := []any{cookie}
	if htmx {
		options = append(options, map[string]string{"HX-Request": "true"})
	}

	recorder := servertest.Get(t, env.Handler, path, options...)

	return recorder.Code, recorder.Body.String()
}
