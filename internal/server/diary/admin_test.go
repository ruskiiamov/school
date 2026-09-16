package diary_test

import (
	"html"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestAdminDiarySearchAndOpen(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	admin := servertest.Login(t, f.env.Handler)

	gone := createUser(t, f.env, auth.RoleStudent, "gone", "Иванова Ушедшая Петровна")
	require.NoError(t, f.env.Auth.SetUserActive(ctx, gone, false))

	algebra := f.openLesson(t, f.class, f.algebra, f.today)
	_, err := f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 5})
	require.NoError(t, err)

	code, body := get(t, f.env, "/admin/diary", admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Введите ФИО или выберите класс")
	assert.Contains(t, body, `href="/admin/diary" aria-current="page"`)

	code, body = get(t, f.env, "/admin/diary?q=иванова", admin, true)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="diary-search"`)
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, ">7А<")
	assert.Contains(t, body, `href="/admin/diary/`+idValue(f.student)+`?q=%D0%B8%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2%D0%B0&amp;year=`+f.year()+`"`)
	assert.NotContains(t, body, "Иванова Ушедшая Петровна")
	assert.NotContains(t, body, "Чужой Ученик")
	assert.NotContains(t, body, "<html")

	code, body = get(t, f.env, "/admin/diary?q=никого", admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Ничего не найдено")

	path := "/admin/diary/" + idValue(f.student)
	code, body = get(t, f.env, path+"?q=иванова", admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Дневник · Иванова Мария Петровна · 7А")
	assert.Contains(t, body, `href="/admin/diary?q=%D0%B8%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2%D0%B0&amp;year=`+f.year()+`"`)
	assert.Contains(t, body, "К поиску")
	assert.Contains(t, body, ">5<")
	assert.Contains(t, body, `name="q" value="иванова"`)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL(path, f.today, url.Values{"lesson": {idValue(algebra)}, "q": {"иванова"}, "year": {f.year()}}))+`" hx-get`)
	assert.NotContains(t, body, "Отсутствовал")

	code, _ = get(t, f.env, "/admin/diary/"+idValue(f.teacher), admin, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, "/admin/diary/"+idValue(gone), admin, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, "/admin/diary/999", admin, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, "/admin/diary/abc", admin, false)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestAdminDiaryHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")

	for _, login := range []string{"teacher", "student", "parent"} {
		cookie := f.env.LoginAs(t, login)

		code, _ := get(t, f.env, "/admin/diary", cookie, false)
		assert.Equal(t, http.StatusNotFound, code, login)
		code, _ = get(t, f.env, "/admin/diary/"+idValue(f.student), cookie, false)
		assert.Equal(t, http.StatusNotFound, code, login)
	}
}
