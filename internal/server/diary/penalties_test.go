package diary_test

import (
	"html"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

type penaltyFixture struct {
	fixture
	lesson   int64
	homework int64
}

func newPenaltyFixture(t *testing.T) penaltyFixture {
	t.Helper()

	f := newFixture(t)
	f.env.EnablePenalties(t)

	homework, err := f.env.Journal.CreateWorkType(t.Context(), f.teacher, f.algebra, "Домашняя работа", true)
	require.NoError(t, err)

	lesson := f.openLesson(t, f.class, f.algebra, f.today)
	f.env.SetPenalty(t, f.teacher, lesson, f.student, homework, "30")

	return penaltyFixture{fixture: f, lesson: lesson, homework: homework}
}

func TestStudentPenaltiesPage(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	f.env.AddWorkoff(t, f.teacher, f.student, "4")
	cookie := f.env.LoginAs(t, "student")

	code, body := get(t, f.env, "/diary/penalties", cookie, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Мытьё полов")
	assert.Contains(t, body, "Долг, м²: 10")
	assert.Contains(t, body, "Алгебра · Домашняя работа")
	assert.Contains(t, body, "30 %")
	assert.Contains(t, body, "частично")
	assert.Contains(t, body, "Сидорова А. А.")
	assert.Contains(t, body, html.EscapeString(diaryURL("/diary", f.today, url.Values{"lesson": {idValue(f.lesson)}})))
	assert.NotContains(t, body, "/delete")
}

func TestStudentDiaryShowsPenalty(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	cookie := f.env.LoginAs(t, "student")

	_, body := get(t, f.env, diaryURL("/diary", f.today, url.Values{"lesson": {idValue(f.lesson)}}), cookie, false)
	assert.Contains(t, body, "Домашняя работа")
	assert.Contains(t, body, "Выполнено 30 % · м²: 14 · не погашено")

	_, body = get(t, f.env, "/", cookie, false)
	assert.Contains(t, body, "Долг, м²: 14")
	assert.Contains(t, body, `href="/diary/penalties"`)

	f.env.AddWorkoff(t, f.teacher, f.student, "14")

	_, body = get(t, f.env, "/", cookie, false)
	assert.NotContains(t, body, "Долг, м²:")
}

func TestParentPenalties(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	parent := createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")
	require.NoError(t, f.env.School.AddChild(t.Context(), parent, f.student))
	cookie := f.env.LoginAs(t, "parent")

	code, body := get(t, f.env, "/diary/penalties", cookie, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Долг, м²: 14")
	assert.Contains(t, body, html.EscapeString(diaryURL("/diary", f.today, url.Values{"child": {idValue(f.student)}, "lesson": {idValue(f.lesson)}})))

	_, body = get(t, f.env, "/", cookie, false)
	assert.Contains(t, body, `href="/diary/penalties?child=`+idValue(f.student)+`"`)

	code, _ = get(t, f.env, "/diary/penalties?child="+idValue(f.stranger), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestAdminDiaryPenalties(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	admin := servertest.Login(t, f.env.Handler)
	path := "/admin/diary/" + idValue(f.student)

	_, body := get(t, f.env, path+"?year="+f.year(), admin, false)
	assert.Contains(t, body, `href="`+html.EscapeString(path+"/penalties?year="+f.year())+`"`)

	code, body := get(t, f.env, path+"/penalties?year="+f.year(), admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Мытьё полов · Иванова Мария Петровна · 7А")
	assert.Contains(t, body, "К дневнику")
	assert.Contains(t, body, "Долг, м²: 14")

	code, _ = get(t, f.env, "/admin/diary/"+idValue(f.teacher)+"/penalties", admin, false)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestDiaryPenaltiesHiddenWhenDisabled(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	f.env.DisablePenalties(t)
	cookie := f.env.LoginAs(t, "student")

	code, _ := get(t, f.env, "/diary/penalties", cookie, false)
	assert.Equal(t, http.StatusNotFound, code)

	_, body := get(t, f.env, diaryURL("/diary", f.today, url.Values{"lesson": {idValue(f.lesson)}}), cookie, false)
	assert.NotContains(t, body, "Выполнено 30 %")
	assert.NotContains(t, body, "Мытьё полов")

	admin := servertest.Login(t, f.env.Handler)
	code, _ = get(t, f.env, "/admin/diary/"+idValue(f.student)+"/penalties", admin, false)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestDiaryPenaltiesOnlyForOwners(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)

	code, _ := get(t, f.env, "/diary/penalties", f.env.LoginAs(t, "teacher"), false)
	assert.Equal(t, http.StatusNotFound, code)
}
