package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/storage"
)

func createClass(t *testing.T, env *testEnv, admin *http.Cookie, year int, name string) int64 {
	t.Helper()

	recorder := postForm(t, env.handler, classesURL(year, ""), url.Values{"name": {name}}, []*http.Cookie{admin}, nil)
	assertRedirect(t, recorder, classesURL(year, ""))

	classes, err := env.school.Classes(t.Context(), year, true)
	require.NoError(t, err)

	for _, class := range classes {
		if class.Name == name {
			return class.ID
		}
	}

	t.Fatalf("class %q not found", name)

	return 0
}

func createPastClass(t *testing.T, env *testEnv, year int, name string) int64 {
	t.Helper()

	id, err := storage.NewClassRepo(env.db).Create(t.Context(), year, name)
	require.NoError(t, err)

	return id
}

func classPathFor(id int64, suffix string) string {
	return "/admin/classes/" + strconv.FormatInt(id, 10) + suffix
}

func classesURL(year int, suffix string) string {
	return "/admin/classes?year=" + strconv.Itoa(year) + suffix
}

func TestClassesEmptyListShowsOnlyCurrentYear(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	body := get(t, env.handler, "/admin/classes", admin).Body.String()
	assert.Contains(t, body, "В "+school.YearName(current)+" пока нет классов")
	assert.Contains(t, body, `href="`+classesURL(current, "")+`" aria-current="page"`)
	assert.NotContains(t, body, school.YearName(current+1))
	assert.NotContains(t, body, school.YearName(current-1))
	assert.Contains(t, body, `action="`+classesURL(current, "")+`"`)
	assert.Contains(t, body, `hx-target="#classes"`)
	assert.Contains(t, body, `href="`+classesURL(current, "&amp;inactive=1")+`"`)
	assert.NotContains(t, body, "required")

	garbage := get(t, env.handler, "/admin/classes?year=abc", admin).Body.String()
	assert.Contains(t, garbage, `href="`+classesURL(current, "")+`" aria-current="page"`)

	years, err := env.school.ClassYears(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int{current}, years)
}

func TestClassesPastYearIsReadOnly(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	past := createPastClass(t, env, current-1, "6А")

	years, err := env.school.ClassYears(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int{current - 1, current}, years)

	body := get(t, env.handler, "/admin/classes", admin).Body.String()
	assert.Less(t, strings.Index(body, school.YearName(current-1)), strings.Index(body, school.YearName(current)))
	assert.Contains(t, body, `href="`+classesURL(current-1, "")+`"`)
	assert.NotContains(t, body, "6А")

	pastPage := get(t, env.handler, classesURL(current-1, ""), admin).Body.String()
	assert.Contains(t, pastPage, ">6А</a>")
	assert.NotContains(t, pastPage, `action="`+classesURL(current-1, "")+`"`)
	assert.NotContains(t, pastPage, "Новый класс")
	assert.Contains(t, pastPage, `href="`+classesURL(current-1, "&amp;edit="+strconv.FormatInt(past, 10))+`"`)

	forged := postForm(t, env.handler, classesURL(current-1, ""), url.Values{"name": {"6Б"}}, []*http.Cookie{admin}, nil)
	assertRedirect(t, forged, classesURL(current-1, ""))

	pastClasses, err := env.school.Classes(t.Context(), current-1, true)
	require.NoError(t, err)
	require.Len(t, pastClasses, 1)

	currentClasses, err := env.school.Classes(t.Context(), current, true)
	require.NoError(t, err)
	require.Len(t, currentClasses, 1)
	assert.Equal(t, "6Б", currentClasses[0].Name)

	assertRedirect(t, postForm(t, env.handler, classPathFor(past, "?year="+strconv.Itoa(current-1)), url.Values{"name": {"6В"}}, []*http.Cookie{admin}, nil), classesURL(current-1, ""))

	class, err := env.school.ClassByID(t.Context(), past)
	require.NoError(t, err)
	assert.Equal(t, "6В", class.Name)
	assert.Equal(t, current-1, class.Year)
}

func TestClassCreateInSelectedYear(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	messy := postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"  7 А "}}, []*http.Cookie{admin}, nil)
	assertRedirect(t, messy, classesURL(current, ""))

	classes, err := env.school.Classes(t.Context(), current, false)
	require.NoError(t, err)
	require.Len(t, classes, 1)
	assert.Equal(t, "7 А", classes[0].Name)
	assert.Equal(t, current, classes[0].Year)
	assert.True(t, classes[0].Active)

	id := classes[0].ID

	list := get(t, env.handler, classesURL(current, ""), admin).Body.String()
	assert.Contains(t, list, `href="`+classPathFor(id, "")+`"`)
	assert.Contains(t, list, ">7 А</a>")
	assert.Contains(t, list, `href="`+classesURL(current, "&amp;edit="+strconv.FormatInt(id, 10))+`"`)
	assert.Contains(t, list, `action="`+classPathFor(id, "/deactivate?year="+strconv.Itoa(current))+`"`)
	assert.NotContains(t, list, "пока нет классов")

	past := createPastClass(t, env, current-1, "7 А")
	assert.NotEqual(t, id, past)

	other := get(t, env.handler, classesURL(current-1, ""), admin).Body.String()
	assert.Contains(t, other, ">7 А</a>")
	assert.NotContains(t, other, `href="`+classPathFor(id, "")+`"`)
}

func TestClassCardShowsEmptyBlocks(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	id := createClass(t, env, admin, current, "7А")

	card := get(t, env.handler, classPathFor(id, ""), admin)
	assert.Equal(t, http.StatusOK, card.Code)
	assert.Contains(t, card.Body.String(), "7А · "+school.YearName(current))
	assert.Contains(t, card.Body.String(), "Пока нет учеников")
	assert.Contains(t, card.Body.String(), "Предметы пока не назначены")
	assert.NotContains(t, card.Body.String(), "Изменить")
	assert.NotContains(t, card.Body.String(), "удалён")
}

func TestClassCreateValidation(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	createClass(t, env, admin, current, "7А")

	duplicate := postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.Contains(t, duplicate.Body.String(), "<html")
	assert.Contains(t, duplicate.Body.String(), "Такой класс в этом году уже есть")
	assert.Contains(t, duplicate.Body.String(), `value="7А"`)

	blank := postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"   "}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "Укажите название")

	unknownYear := postForm(t, env.handler, classesURL(1999, ""), url.Values{"name": {"8А"}}, []*http.Cookie{admin}, nil)
	assertRedirect(t, unknownYear, classesURL(current, ""))

	classes, err := env.school.Classes(t.Context(), current, true)
	require.NoError(t, err)
	require.Len(t, classes, 2)

	years, err := env.school.ClassYears(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int{current}, years)
}

func TestClassEditModeShowsFormForOneRow(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	a := createClass(t, env, admin, current, "7А")
	b := createClass(t, env, admin, current, "7Б")

	body := get(t, env.handler, classesURL(current, "&edit="+strconv.FormatInt(a, 10)), admin).Body.String()
	assert.Contains(t, body, `value="7А"`)
	assert.Contains(t, body, `action="`+classPathFor(a, "?year="+strconv.Itoa(current))+`"`)
	assert.Contains(t, body, `href="`+classesURL(current, "")+`"`)
	assert.Contains(t, body, ">Отмена</a>")
	assert.NotContains(t, body, `value="7Б"`)
	assert.Contains(t, body, `href="`+classesURL(current, "&amp;edit="+strconv.FormatInt(b, 10))+`"`)

	fragment := get(t, env.handler, classesURL(current, "&inactive=1&edit="+strconv.FormatInt(a, 10)), admin, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.NotContains(t, fragment.Body.String(), "<html")
	assert.Contains(t, fragment.Body.String(), `id="classes"`)
	assert.Contains(t, fragment.Body.String(), `action="`+classPathFor(a, "?year="+strconv.Itoa(current)+"&amp;inactive=1")+`"`)
}

func TestClassRenameAndValidation(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	id := createClass(t, env, admin, current, "7А")
	createClass(t, env, admin, current, "7Б")

	renamed := postForm(t, env.handler, classPathFor(id, "?year="+strconv.Itoa(current)), url.Values{"name": {"7В"}}, []*http.Cookie{admin}, nil)
	assertRedirect(t, renamed, classesURL(current, ""))

	class, err := env.school.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "7В", class.Name)
	assert.Equal(t, current, class.Year)

	taken := postForm(t, env.handler, classPathFor(id, "?year="+strconv.Itoa(current)), url.Values{"name": {"7Б"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.Contains(t, taken.Body.String(), "Такой класс в этом году уже есть")
	assert.Equal(t, 1, strings.Count(taken.Body.String(), `value="7Б"`))
	assert.Contains(t, taken.Body.String(), ">7Б</a>")

	blank := postForm(t, env.handler, classPathFor(id, "?year="+strconv.Itoa(current)), url.Values{"name": {""}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "Укажите название")

	class, err = env.school.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "7В", class.Name)
}

func TestClassDeactivateHidesButKeepsName(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()
	year := strconv.Itoa(current)

	id := createClass(t, env, admin, current, "7А")

	assertRedirect(t, postForm(t, env.handler, classPathFor(id, "/deactivate?year="+year+"&inactive=1"), nil, []*http.Cookie{admin}, nil), classesURL(current, "&inactive=1"))

	visible := get(t, env.handler, classesURL(current, ""), admin).Body.String()
	assert.NotContains(t, visible, "7А")
	assert.Contains(t, visible, "Показать удалённые")

	all := get(t, env.handler, classesURL(current, "&inactive=1"), admin).Body.String()
	assert.Contains(t, all, "7А")
	assert.Contains(t, all, "удалён")
	assert.Contains(t, all, "Скрыть удалённые")
	assert.Contains(t, all, `href="`+classesURL(current, "")+`"`)
	assert.Contains(t, all, `action="`+classPathFor(id, "/activate?year="+year+"&amp;inactive=1")+`"`)
	assert.Contains(t, all, `action="`+classesURL(current, "&amp;inactive=1")+`"`)

	card := get(t, env.handler, classPathFor(id, ""), admin).Body.String()
	assert.Contains(t, card, "удалён")

	duplicate := postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.Contains(t, duplicate.Body.String(), "Такой класс в этом году уже есть")

	assertRedirect(t, postForm(t, env.handler, classPathFor(id, "/activate?year="+year), nil, []*http.Cookie{admin}, nil), classesURL(current, ""))

	class, err := env.school.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.True(t, class.Active)
}

func TestClassHtmxRequestsGetListFragment(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()
	htmx := map[string]string{"HX-Request": "true"}

	created := postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, created.Code)
	assert.NotContains(t, created.Body.String(), "<html")
	assert.Contains(t, created.Body.String(), `id="classes"`)
	assert.Contains(t, created.Body.String(), ">7А</a>")

	duplicate := postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.NotContains(t, duplicate.Body.String(), "<html")
	assert.Contains(t, duplicate.Body.String(), "Такой класс в этом году уже есть")
}

func TestClassUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)

	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/admin/classes/999", admin).Code)
	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/admin/classes/abc", admin).Code)
	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/admin/classes/new", admin).Code)
	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, "/admin/classes/999", url.Values{"name": {"X"}}, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, "/admin/classes/999/deactivate", nil, []*http.Cookie{admin}, nil).Code)

	_, err := env.school.ClassByID(t.Context(), 999)
	assert.ErrorIs(t, err, school.ErrNotFound)
}

func TestClassesHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.createUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.loginAs(t, "teacher")
	current := env.school.CurrentYear()

	assertRedirect(t, get(t, env.handler, "/admin/classes"), "/login")
	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/admin/classes", teacher).Code)
	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{teacher}, nil).Code)

	classes, err := env.school.Classes(t.Context(), current, true)
	require.NoError(t, err)
	assert.Empty(t, classes)
}
