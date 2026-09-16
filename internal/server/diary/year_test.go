package diary_test

import (
	"html"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/storage"
)

type pastFixture struct {
	fixture
	past      int
	pastClass int64
	lesson    int64
}

func newPastFixture(t *testing.T) pastFixture {
	t.Helper()

	f := pastFixture{fixture: newFixture(t)}
	ctx := t.Context()
	f.past = f.env.School.CurrentYear() - 1

	var err error
	f.pastClass, err = storage.NewClassRepo(f.env.DB).Create(ctx, f.past, "6А")
	require.NoError(t, err)
	require.NoError(t, storage.NewClassStudentRepo(f.env.DB).Add(ctx, f.pastClass, f.student))

	start, _ := f.env.School.YearBounds(f.past)
	f.lesson, err = storage.NewLessonRepo(f.env.DB).Create(ctx, storage.Lesson{
		ClassID: f.pastClass, SubjectID: f.algebra, TeacherID: f.teacher, Date: start.AddDate(0, 1, 15),
	})
	require.NoError(t, err)

	_, err = f.env.Journal.AddMark(ctx, f.teacher, f.lesson, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 4})
	require.NoError(t, err)

	return f
}

func TestStudentMarksPastYear(t *testing.T) {
	t.Parallel()

	f := newPastFixture(t)
	student := f.env.LoginAs(t, "student")
	current := f.env.School.CurrentYear()
	year := strconv.Itoa(f.past)
	start, _ := f.env.School.YearBounds(f.past)

	code, body := get(t, f.env, "/diary/summary", student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `<option value="`+strconv.Itoa(current)+`" selected>`+school.YearName(current)+`</option>`)
	assert.Contains(t, body, `<option value="`+year+`">`+school.YearName(f.past)+`</option>`)
	assert.Contains(t, body, "Оценок за период нет")

	query := url.Values{"year": {year}, "from": {start.Format("2006-01-02")}, "to": {start.AddDate(0, 2, 0).Format("2006-01-02")}}
	code, body = get(t, f.env, "/diary/summary?"+query.Encode(), student, true)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `<option value="`+year+`" selected>`)
	assert.Contains(t, body, "Алгебра")
	assert.Contains(t, body, ">4</a>")
	assert.Contains(t, body, html.EscapeString(diaryURL("/diary", start.AddDate(0, 1, 15), url.Values{"lesson": {idValue(f.lesson)}, "year": {year}})))
	assert.Contains(t, body, "year="+year)

	code, body = get(t, f.env, "/diary/summary?year="+year, student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `name="from" value="`+start.Format("2006-01-02")+`"`)
}

func TestAdminDiaryByYearAndClass(t *testing.T) {
	t.Parallel()

	f := newPastFixture(t)
	admin := servertest.Login(t, f.env.Handler)
	year := strconv.Itoa(f.past)
	filter := url.Values{"year": {year}, "class": {idValue(f.pastClass)}}

	code, body := get(t, f.env, "/admin/diary", admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `name="year"`)
	assert.Contains(t, body, ">7А</option>")
	assert.Contains(t, body, ">9Б</option>")
	assert.NotContains(t, body, ">6А</option>")

	code, body = get(t, f.env, "/admin/diary?class="+idValue(f.class), admin, true)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.NotContains(t, body, "Чужой Ученик")
	assert.Contains(t, body, `href="/admin/diary/`+idValue(f.student)+`?class=`+idValue(f.class)+`&amp;year=`+strconv.Itoa(f.env.School.CurrentYear())+`"`)

	code, body = get(t, f.env, "/admin/diary?"+filter.Encode(), admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `<option value="`+year+`" selected>`)
	assert.Contains(t, body, ` selected>6А</option>`)
	assert.NotContains(t, body, ">7А</option>")
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, ">6А<")
	assert.Contains(t, body, `href="`+html.EscapeString("/admin/diary/"+idValue(f.student)+"?"+filter.Encode())+`"`)

	code, body = get(t, f.env, "/admin/diary?"+url.Values{"year": {year}, "class": {idValue(f.class)}}.Encode(), admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Введите ФИО или выберите класс")

	path := "/admin/diary/" + idValue(f.student)
	code, body = get(t, f.env, path+"?"+filter.Encode(), admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Дневник · Иванова Мария Петровна · 6А")
	assert.Contains(t, body, `href="`+html.EscapeString("/admin/diary?"+filter.Encode())+`"`)
	assert.Contains(t, body, `href="`+html.EscapeString(path+"/summary?"+filter.Encode())+`"`)

	start, _ := f.env.School.YearBounds(f.past)
	summary := url.Values{"year": {year}, "class": {idValue(f.pastClass)}, "from": {start.Format("2006-01-02")}, "to": {start.AddDate(0, 2, 0).Format("2006-01-02")}}
	code, body = get(t, f.env, path+"/summary?"+summary.Encode(), admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Оценки · Иванова Мария Петровна · 6А")
	assert.Contains(t, body, ">4</a>")
	assert.Contains(t, body, `href="`+html.EscapeString("/admin/diary/"+idValue(f.student)+"?"+filter.Encode())+`"`)
	assert.Contains(t, body, `<option value="`+year+`" selected>`)

	for _, login := range []string{"teacher", "student"} {
		code, _ = get(t, f.env, "/admin/diary?"+filter.Encode(), f.env.LoginAs(t, login), false)
		assert.Equal(t, http.StatusNotFound, code, login)
	}

}
