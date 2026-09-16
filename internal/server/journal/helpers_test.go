package journal_test

import (
	"net/http"
	"regexp"
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

var lessonLocation = regexp.MustCompile(`^/journal/lessons/(\d+)$`)

type fixture struct {
	env     *servertest.Env
	teacher int64
	class   int64
	subject int64
	today   time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	env := servertest.New(t)

	class, err := env.School.CreateClass(t.Context(), "7А")
	require.NoError(t, err)

	subject, err := env.School.CreateSubject(t.Context(), school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	teacher := createUser(t, env, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")

	return fixture{env: env, teacher: teacher, class: class, subject: subject, today: env.School.Today()}
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

func pairValue(classID, subjectID int64) string {
	return strconv.FormatInt(classID, 10) + "-" + strconv.FormatInt(subjectID, 10)
}

func dateValue(t time.Time) string {
	return t.Format(validation.DateLayout)
}

func lessonPath(id int64, suffix string) string {
	return "/journal/lessons/" + strconv.FormatInt(id, 10) + suffix
}

func openLesson(t *testing.T, env *servertest.Env, cookie *http.Cookie, classID, subjectID int64, date time.Time) int64 {
	t.Helper()

	recorder := servertest.PostForm(t, env.Handler, "/journal",
		map[string][]string{"pair": {pairValue(classID, subjectID)}, "date": {dateValue(date)}},
		[]*http.Cookie{cookie}, nil)
	require.Equal(t, http.StatusSeeOther, recorder.Code, recorder.Body.String())

	match := lessonLocation.FindStringSubmatch(recorder.Header().Get("Location"))
	require.NotNil(t, match, recorder.Header().Get("Location"))

	id, err := strconv.ParseInt(match[1], 10, 64)
	require.NoError(t, err)

	return id
}

func journalMarkInput(workTypeID int64, value int) journal.MarkInput {
	return journal.MarkInput{WorkTypeID: workTypeID, Value: value}
}

func journalHomework(text string) journal.HomeworkInput {
	return journal.HomeworkInput{Text: text}
}
