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
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestTeacherSummaryGrid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, petr))

	workTypes, err := f.env.School.WorkTypes(ctx, false)
	require.NoError(t, err)

	body := servertest.Get(t, f.env.Handler, "/journal/summary", teacher).Body.String()
	assert.Contains(t, body, `href="/journal/summary" aria-current="page"`)
	assert.Contains(t, body, "Выберите класс и предмет")
	assert.Contains(t, body, `name="from" value="`+dateValue(monthStart(f.today))+`"`)
	assert.Contains(t, body, ">Этот месяц<")

	first := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	second := openLesson(t, f.env, teacher, f.class, f.subject, f.today.AddDate(0, 0, 1))
	_, err = f.env.Journal.AddMark(ctx, f.teacher, first, maria, journalMarkInput(workTypes[0].ID, 5))
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, first, maria, journalMarkInput(workTypes[0].ID, 4))
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, second, maria, journalMarkInput(workTypes[0].ID, 3))
	require.NoError(t, err)
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, first, petr, journal.RecordInput{Absent: true}))
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, second, maria, journal.RecordInput{Absent: true}))

	query := url.Values{"pair": {pairValue(f.class, f.subject)}, "from": {dateValue(f.today)}, "to": {dateValue(f.today.AddDate(0, 0, 1))}}
	fragment := servertest.Get(t, f.env.Handler, "/journal/summary?"+query.Encode(), teacher, map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, fragment.Code)
	body = fragment.Body.String()
	assert.Contains(t, body, `id="summary"`)
	assert.NotContains(t, body, "<html")
	assert.Contains(t, body, `<option value="`+pairValue(f.class, f.subject)+`" selected>7А · Алгебра</option>`)
	assert.Contains(t, body, `href="`+lessonPath(first, "")+`"`)
	assert.Contains(t, body, `href="`+lessonPath(second, "")+`"`)
	assert.Contains(t, body, ">"+f.today.Format("02.01")+"</a>")
	assert.Contains(t, body, `href="`+lessonPath(first, "")+studentQuery(maria)+`" class="inline-flex min-h-9 items-center rounded-md px-2 font-semibold text-indigo-700 hover:bg-indigo-50">5, 4</a>`)
	assert.Contains(t, body, ">3 · Н</a>")
	assert.Contains(t, body, ">Н</a>")
	assert.Contains(t, body, ">4,00</td>")
	assert.Contains(t, body, ">—</td>")
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, "Козлов П.")

	empty := url.Values{"pair": {pairValue(f.class, f.subject)}, "from": {dateValue(f.today.AddDate(0, 0, 10))}, "to": {dateValue(f.today.AddDate(0, 0, 12))}}
	body = servertest.Get(t, f.env.Handler, "/journal/summary?"+empty.Encode(), teacher).Body.String()
	assert.Contains(t, body, "Уроков за период нет")

	foreign := url.Values{"pair": {pairValue(f.class, 999)}}
	body = servertest.Get(t, f.env.Handler, "/journal/summary?"+foreign.Encode(), teacher).Body.String()
	assert.Contains(t, body, "Выберите класс и предмет")
}

func TestAdminSummaryGrid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	admin := servertest.Login(t, f.env.Handler)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))

	workTypes, err := f.env.School.WorkTypes(ctx, false)
	require.NoError(t, err)

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, id, maria, journalMarkInput(workTypes[0].ID, 5))
	require.NoError(t, err)

	pair := url.Values{"class": {strconv.FormatInt(f.class, 10)}, "subject": {strconv.FormatInt(f.subject, 10)}}
	body := servertest.Get(t, f.env.Handler, "/admin/journal?"+pair.Encode(), admin).Body.String()
	assert.Contains(t, body, `href="`+html.EscapeString("/admin/journal/summary?"+pair.Encode()+"&year="+f.year())+`"`)
	assert.Contains(t, body, ">Сводка</a>")

	query := pair
	query.Set("from", dateValue(f.today))
	query.Set("to", dateValue(f.today))
	body = servertest.Get(t, f.env.Handler, "/admin/journal/summary?"+query.Encode(), admin).Body.String()
	assert.Contains(t, body, `href="/admin/journal" aria-current="page"`)
	assert.Contains(t, body, `href="/admin/journal/lessons/`+strconv.FormatInt(id, 10)+`?student=`+strconv.FormatInt(maria, 10)+`"`)
	assert.Contains(t, body, ">5</a>")
	assert.Contains(t, body, ">5,00</td>")

	body = servertest.Get(t, f.env.Handler, "/admin/journal/summary", admin).Body.String()
	assert.Contains(t, body, "Выберите класс и предмет")

	for _, login := range []string{"teacher", "maria"} {
		cookie := f.env.LoginAs(t, login)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journal/summary", cookie).Code, login)
	}

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/journal/summary", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/journal/summary", f.env.LoginAs(t, "maria")).Code)
}
