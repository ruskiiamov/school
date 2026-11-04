package journal_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/storage"
)

type pastFixture struct {
	fixture
	past      int
	pastClass int64
	lesson    int64
	maria     int64
}

func newPastFixture(t *testing.T) pastFixture {
	t.Helper()

	f := pastFixture{fixture: newFixture(t)}
	ctx := t.Context()
	f.past = f.env.School.CurrentYear() - 1

	var err error
	f.pastClass, err = storage.NewClassRepo(f.env.DB).Create(ctx, f.past, "6А")
	require.NoError(t, err)
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	require.NoError(t, storage.NewAssignmentRepo(f.env.DB).Upsert(ctx, f.pastClass, f.subject, f.teacher))

	f.maria = createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, storage.NewClassStudentRepo(f.env.DB).Add(ctx, f.pastClass, f.maria))

	start, _ := f.env.School.YearBounds(f.past)
	f.lesson, err = storage.NewLessonRepo(f.env.DB).Create(ctx, storage.Lesson{
		ClassID: f.pastClass, SubjectID: f.subject, TeacherID: f.teacher, Date: start.AddDate(0, 1, 15),
	})
	require.NoError(t, err)

	workTypes := f.env.EnsureWorkTypes(t, f.teacher, f.subject)
	f.env.SetMark(t, f.teacher, f.lesson, f.maria, workTypes[0].ID, "5")

	return f
}

func TestPairJournalPastYear(t *testing.T) {
	t.Parallel()

	f := newPastFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	past := strconv.Itoa(f.past)
	pastPair := pairPath(f.pastClass, f.subject)

	home := servertest.Get(t, f.env.Handler, "/", teacher).Body.String()
	assert.Contains(t, home, `href="`+pairPath(f.class, f.subject)+`"`)
	assert.NotContains(t, home, `href="`+pastPair+`"`)

	home = servertest.Get(t, f.env.Handler, "/?year="+past, teacher).Body.String()
	assert.Contains(t, home, `<option value="`+past+`" selected>`+school.YearName(f.past)+`</option>`)
	assert.Contains(t, home, `href="`+pastPair+`"`)
	assert.NotContains(t, home, `href="`+pairPath(f.class, f.subject)+`"`)

	body := servertest.Get(t, f.env.Handler, pastPair, teacher).Body.String()
	assert.Contains(t, body, "6А · Алгебра · "+school.YearName(f.past))
	assert.Contains(t, body, `href="/?year=`+past+`"`)
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(f.lesson, 10)+`"`)
	assert.Contains(t, body, `: 5">5`)

	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, lessonPath(f.lesson, ""), teacher).Code)
}

func TestAdminJournalsPastYear(t *testing.T) {
	t.Parallel()

	f := newPastFixture(t)
	admin := servertest.Login(t, f.env.Handler)
	year := strconv.Itoa(f.past)

	body := servertest.Get(t, f.env.Handler, "/admin/journals", admin).Body.String()
	assert.Contains(t, body, ">7А</option>")
	assert.NotContains(t, body, ">6А</option>")

	pair := url.Values{"year": {year}, "class": {strconv.FormatInt(f.pastClass, 10)}, "subject": {strconv.FormatInt(f.subject, 10)}}
	body = servertest.Get(t, f.env.Handler, "/admin/journals?"+pair.Encode(), admin).Body.String()
	assert.Contains(t, body, `<option value="`+year+`" selected>`)
	assert.Contains(t, body, ` selected>6А</option>`)
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(f.lesson, 10)+`"`)
	assert.Contains(t, body, `: 5">5`)
}
