package diary_test

import (
	"html"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

func periodQuery(from, to time.Time, params url.Values) string {
	query := url.Values{"from": {from.Format(validation.DateLayout)}, "to": {to.Format(validation.DateLayout)}}
	for name, values := range params {
		query[name] = values
	}

	return query.Encode()
}

func TestStudentMarksSummary(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	student := f.env.LoginAs(t, "student")

	code, body := get(t, f.env, "/diary/summary", student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `href="/diary/summary" aria-current="page"`)
	assert.Contains(t, body, "Оценок за период нет")
	assert.Contains(t, body, ">Этот месяц<")

	algebra := f.openLesson(t, f.class, f.algebra, f.today)
	history := f.openLesson(t, f.class, f.history, f.today.AddDate(0, 0, 1))
	_, err := f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 5, Label: "у доски"})
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 4})
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, history, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 3})
	require.NoError(t, err)
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, history, f.student, journal.RecordInput{Absent: true}))

	path := "/diary/summary?" + periodQuery(f.today, f.today.AddDate(0, 0, 1), nil)
	code, body = get(t, f.env, path, student, true)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="marks"`)
	assert.NotContains(t, body, "<html")
	assert.Less(t, indexOf(body, ">Алгебра<"), indexOf(body, ">История<"))
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today, url.Values{"lesson": {idValue(algebra)}, "year": {f.year()}}))+`" title="`+f.today.Format("02.01.2006")+" · Ответ на уроке · у доски"+`"`)
	assert.Contains(t, body, ">5</a>")
	assert.Contains(t, body, ">4</a>")
	assert.Contains(t, body, ">3</a>")
	assert.Contains(t, body, "Средний: 4,50")
	assert.Contains(t, body, "Средний: 3,00")
	assert.Contains(t, body, "Н: 1")
	assert.Contains(t, body, "Н: 0")

	code, body = get(t, f.env, "/diary/summary?"+periodQuery(f.today.AddDate(0, 0, 5), f.today.AddDate(0, 0, 6), nil), student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Оценок за период нет")

	code, _ = get(t, f.env, "/diary/summary?child="+idValue(f.stranger), student, false)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestParentAndAdminMarksSummary(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	parent := createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")
	createUser(t, f.env, auth.RoleParent, "childless", "Бездетный Родитель Иванович")
	require.NoError(t, f.env.School.AddChild(ctx, parent, f.student))

	algebra := f.openLesson(t, f.class, f.algebra, f.today)
	_, err := f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 5})
	require.NoError(t, err)

	cookie := f.env.LoginAs(t, "parent")
	code, body := get(t, f.env, "/diary/summary", cookie, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `href="`+html.EscapeString("/diary/summary?child="+idValue(f.student)+"&from=")+``)
	assert.Contains(t, body, `aria-current="page"`)
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, ">5</a>")
	assert.Contains(t, body, `name="child" value="`+idValue(f.student)+`"`)

	code, _ = get(t, f.env, "/diary/summary?child="+idValue(f.stranger), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)

	code, body = get(t, f.env, "/diary/summary", f.env.LoginAs(t, "childless"), false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "К вашему аккаунту не привязаны дети")

	admin := servertest.Login(t, f.env.Handler)
	code, body = get(t, f.env, "/admin/diary/"+idValue(f.student)+"?q=иванова", admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `href="/admin/diary/`+idValue(f.student)+`/summary?q=%D0%B8%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2%D0%B0&amp;year=`+f.year()+`"`)
	assert.Contains(t, body, ">Оценки</a>")

	code, body = get(t, f.env, "/admin/diary/"+idValue(f.student)+"/summary?q=иванова", admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Оценки · Иванова Мария Петровна · 7А")
	assert.Contains(t, body, `href="/admin/diary/`+idValue(f.student)+`?q=%D0%B8%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2%D0%B0&amp;year=`+f.year()+`"`)
	assert.Contains(t, body, "К дневнику")
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/admin/diary/"+idValue(f.student), f.today, url.Values{"lesson": {idValue(algebra)}, "q": {"иванова"}, "year": {f.year()}}))+`"`)
	assert.Contains(t, body, ">5</a>")

	code, _ = get(t, f.env, "/admin/diary/"+idValue(f.teacher)+"/summary", admin, false)
	assert.Equal(t, http.StatusNotFound, code)

	for _, login := range []string{"teacher", "student", "parent"} {
		code, _ = get(t, f.env, "/admin/diary/"+idValue(f.student)+"/summary", f.env.LoginAs(t, login), false)
		assert.Equal(t, http.StatusNotFound, code, login)
	}

	code, _ = get(t, f.env, "/diary/summary", f.env.LoginAs(t, "teacher"), false)
	assert.Equal(t, http.StatusNotFound, code)
}
