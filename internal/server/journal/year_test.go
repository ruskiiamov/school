package journal_test

import (
	"html"
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

	workTypes, err := f.env.School.WorkTypes(ctx, false)
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, f.lesson, f.maria, journalMarkInput(workTypes[0].ID, 5))
	require.NoError(t, err)

	return f
}

func TestTeacherSummaryPastYear(t *testing.T) {
	t.Parallel()

	f := newPastFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	current := f.env.School.CurrentYear()
	start, _ := f.env.School.YearBounds(f.past)

	body := servertest.Get(t, f.env.Handler, "/journal/summary", teacher).Body.String()
	assert.Contains(t, body, `<option value="`+strconv.Itoa(current)+`" selected>`+school.YearName(current)+`</option>`)
	assert.Contains(t, body, `<option value="`+strconv.Itoa(f.past)+`">`+school.YearName(f.past)+`</option>`)
	assert.Contains(t, body, "7А · Алгебра")
	assert.NotContains(t, body, "6А · Алгебра")

	query := url.Values{"year": {strconv.Itoa(f.past)}}
	body = servertest.Get(t, f.env.Handler, "/journal/summary?"+query.Encode(), teacher).Body.String()
	assert.Contains(t, body, `<option value="`+strconv.Itoa(f.past)+`" selected>`)
	assert.Contains(t, body, "6А · Алгебра")
	assert.NotContains(t, body, "7А · Алгебра")
	assert.Contains(t, body, `name="from" value="`+dateValue(start)+`"`)
	assert.Contains(t, body, "year="+strconv.Itoa(f.past))

	query.Set("pair", pairValue(f.pastClass, f.subject))
	query.Set("from", dateValue(start))
	query.Set("to", dateValue(start.AddDate(0, 2, 0)))
	body = servertest.Get(t, f.env.Handler, "/journal/summary?"+query.Encode(), teacher).Body.String()
	assert.Contains(t, body, `href="`+lessonPath(f.lesson, "")+studentQuery(f.maria)+`"`)
	assert.Contains(t, body, ">5</a>")

	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, lessonPath(f.lesson, ""), teacher).Code)

	unknown := servertest.Get(t, f.env.Handler, "/journal/summary?year=1999", teacher).Body.String()
	assert.Contains(t, unknown, `<option value="`+strconv.Itoa(current)+`" selected>`)

	next := servertest.Get(t, f.env.Handler, "/journal/summary?year="+strconv.Itoa(current+1), teacher).Body.String()
	assert.Contains(t, next, `<option value="`+strconv.Itoa(current+1)+`" selected>`+school.YearName(current+1)+`</option>`)
	assert.Contains(t, next, "В этом году вам не назначены классы и предметы")
}

func TestAdminJournalPastYear(t *testing.T) {
	t.Parallel()

	f := newPastFixture(t)
	admin := servertest.Login(t, f.env.Handler)
	year := strconv.Itoa(f.past)
	start, _ := f.env.School.YearBounds(f.past)

	body := servertest.Get(t, f.env.Handler, "/admin/journal", admin).Body.String()
	assert.Contains(t, body, `name="year"`)
	assert.Contains(t, body, ">7А</option>")
	assert.NotContains(t, body, ">6А</option>")

	pair := url.Values{"year": {year}, "class": {strconv.FormatInt(f.pastClass, 10)}, "subject": {strconv.FormatInt(f.subject, 10)}}
	body = servertest.Get(t, f.env.Handler, "/admin/journal?"+pair.Encode(), admin).Body.String()
	assert.Contains(t, body, `<option value="`+year+`" selected>`)
	assert.Contains(t, body, ` selected>6А</option>`)
	assert.Contains(t, body, `href="/admin/journal/lessons/`+strconv.FormatInt(f.lesson, 10)+`"`)
	assert.Contains(t, body, `href="`+html.EscapeString("/admin/journal/summary?"+pair.Encode())+`"`)

	lesson := servertest.Get(t, f.env.Handler, "/admin/journal/lessons/"+strconv.FormatInt(f.lesson, 10), admin).Body.String()
	assert.Contains(t, lesson, `href="`+html.EscapeString("/admin/journal?"+pair.Encode())+`"`)

	summary := pair
	summary.Set("from", dateValue(start))
	summary.Set("to", dateValue(start.AddDate(0, 2, 0)))
	body = servertest.Get(t, f.env.Handler, "/admin/journal/summary?"+summary.Encode(), admin).Body.String()
	assert.Contains(t, body, `<option value="`+year+`" selected>`)
	assert.Contains(t, body, ">5</a>")
	assert.Contains(t, body, "year="+year)
}
