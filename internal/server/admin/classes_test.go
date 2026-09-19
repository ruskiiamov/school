package admin_test

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
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/storage"
)

func createClass(t *testing.T, env *servertest.Env, admin *http.Cookie, year int, name string) int64 {
	t.Helper()

	recorder := servertest.PostForm(t, env.Handler, classesURL(year, ""), url.Values{"name": {name}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, recorder, classesURL(year, ""))

	classes, err := env.School.Classes(t.Context(), year, true)
	require.NoError(t, err)

	for _, class := range classes {
		if class.Name == name {
			return class.ID
		}
	}

	t.Fatalf("class %q not found", name)

	return 0
}

func createPastClass(t *testing.T, env *servertest.Env, year int, name string) int64 {
	t.Helper()

	id, err := storage.NewClassRepo(env.DB).Create(t.Context(), year, name)
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

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	body := servertest.Get(t, env.Handler, "/admin/classes", admin).Body.String()
	assert.Contains(t, body, "В "+school.YearName(current)+" пока нет классов")
	assert.Contains(t, body, `href="`+classesURL(current, "")+`" aria-current="page"`)
	assert.Contains(t, body, `href="`+classesURL(current+1, "")+`"`)
	assert.NotContains(t, body, school.YearName(current-1))
	assert.Contains(t, body, `action="`+classesURL(current, "")+`"`)
	assert.Contains(t, body, `hx-target="#classes"`)
	assert.Contains(t, body, `href="`+classesURL(current, "&amp;inactive=1")+`"`)
	assert.NotContains(t, body, "required")

	garbage := servertest.Get(t, env.Handler, "/admin/classes?year=abc", admin).Body.String()
	assert.Contains(t, garbage, `href="`+classesURL(current, "")+`" aria-current="page"`)

	years, err := env.School.ClassYears(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int{current, current + 1}, years)
}

func TestClassesPastYearIsReadOnly(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	past := createPastClass(t, env, current-1, "6А")

	years, err := env.School.ClassYears(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int{current - 1, current, current + 1}, years)

	body := servertest.Get(t, env.Handler, "/admin/classes", admin).Body.String()
	assert.Less(t, strings.Index(body, school.YearName(current-1)), strings.Index(body, school.YearName(current)))
	assert.Contains(t, body, `href="`+classesURL(current-1, "")+`"`)
	assert.NotContains(t, body, "6А")

	pastPage := servertest.Get(t, env.Handler, classesURL(current-1, ""), admin).Body.String()
	assert.Contains(t, pastPage, ">6А</span>")
	assert.NotContains(t, pastPage, `action="`+classesURL(current-1, "")+`"`)
	assert.NotContains(t, pastPage, "Новый класс")
	assert.Contains(t, pastPage, `href="`+classesURL(current-1, "&amp;edit="+strconv.FormatInt(past, 10))+`"`)

	forged := servertest.PostForm(t, env.Handler, classesURL(current-1, ""), url.Values{"name": {"6Б"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, forged, classesURL(current-1, ""))

	pastClasses, err := env.School.Classes(t.Context(), current-1, true)
	require.NoError(t, err)
	require.Len(t, pastClasses, 1)

	currentClasses, err := env.School.Classes(t.Context(), current, true)
	require.NoError(t, err)
	require.Len(t, currentClasses, 1)
	assert.Equal(t, "6Б", currentClasses[0].Name)

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(past, "?year="+strconv.Itoa(current-1)), url.Values{"name": {"6В"}}, []*http.Cookie{admin}, nil), classesURL(current-1, ""))

	class, err := env.School.ClassByID(t.Context(), past)
	require.NoError(t, err)
	assert.Equal(t, "6В", class.Name)
	assert.Equal(t, current-1, class.Year)
}

func TestClassCreateInSelectedYear(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	messy := servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"  7 А "}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, messy, classesURL(current, ""))

	classes, err := env.School.Classes(t.Context(), current, false)
	require.NoError(t, err)
	require.Len(t, classes, 1)
	assert.Equal(t, "7 А", classes[0].Name)
	assert.Equal(t, current, classes[0].Year)
	assert.True(t, classes[0].Active)

	id := classes[0].ID

	list := servertest.Get(t, env.Handler, classesURL(current, ""), admin).Body.String()
	assert.Contains(t, list, `href="`+classPathFor(id, "")+`"`)
	assert.Contains(t, list, ">7 А</span>")
	assert.Contains(t, list, `href="`+classesURL(current, "&amp;edit="+strconv.FormatInt(id, 10))+`"`)
	assert.NotContains(t, list, `formaction="`+classPathFor(id, "/deactivate?year="+strconv.Itoa(current))+`"`)
	assert.NotContains(t, list, "пока нет классов")

	editing := servertest.Get(t, env.Handler, classesURL(current, "&edit="+strconv.FormatInt(id, 10)), admin).Body.String()
	assert.Contains(t, editing, `formaction="`+classPathFor(id, "/deactivate?year="+strconv.Itoa(current))+`"`)

	past := createPastClass(t, env, current-1, "7 А")
	assert.NotEqual(t, id, past)

	other := servertest.Get(t, env.Handler, classesURL(current-1, ""), admin).Body.String()
	assert.Contains(t, other, ">7 А</span>")
	assert.NotContains(t, other, `href="`+classPathFor(id, "")+`"`)
}

func TestClassCardShowsEmptyBlocks(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	id := createClass(t, env, admin, current, "7А")

	card := servertest.Get(t, env.Handler, classPathFor(id, ""), admin)
	assert.Equal(t, http.StatusOK, card.Code)
	assert.Contains(t, card.Body.String(), "7А · "+school.YearName(current))
	assert.Contains(t, card.Body.String(), "Пока нет учеников")
	assert.Contains(t, card.Body.String(), "Предметы пока не назначены")
	assert.NotContains(t, card.Body.String(), "Изменить")
	assert.NotContains(t, card.Body.String(), "удалён")
}

func TestClassCreateValidation(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	createClass(t, env, admin, current, "7А")

	duplicate := servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.Contains(t, duplicate.Body.String(), "<html")
	assert.Contains(t, duplicate.Body.String(), "Такой класс в этом году уже есть")
	assert.Contains(t, duplicate.Body.String(), `value="7А"`)

	blank := servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"   "}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "Укажите название")

	unknownYear := servertest.PostForm(t, env.Handler, classesURL(1999, ""), url.Values{"name": {"8А"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, unknownYear, classesURL(current, ""))

	classes, err := env.School.Classes(t.Context(), current, true)
	require.NoError(t, err)
	require.Len(t, classes, 2)

	years, err := env.School.ClassYears(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int{current, current + 1}, years)
}

func TestClassEditModeShowsFormForOneRow(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	a := createClass(t, env, admin, current, "7А")
	b := createClass(t, env, admin, current, "7Б")

	body := servertest.Get(t, env.Handler, classesURL(current, "&edit="+strconv.FormatInt(a, 10)), admin).Body.String()
	assert.Contains(t, body, `value="7А"`)
	assert.Contains(t, body, `action="`+classPathFor(a, "?year="+strconv.Itoa(current))+`"`)
	assert.Contains(t, body, `data-cancel="`+classesURL(current, "")+`"`)
	assert.NotContains(t, body, `aria-label="Отмена"`)
	assert.NotContains(t, body, `value="7Б"`)
	assert.Contains(t, body, `href="`+classesURL(current, "&amp;edit="+strconv.FormatInt(b, 10))+`"`)

	fragment := servertest.Get(t, env.Handler, classesURL(current, "&inactive=1&edit="+strconv.FormatInt(a, 10)), admin, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.NotContains(t, fragment.Body.String(), "<html")
	assert.Contains(t, fragment.Body.String(), `id="classes"`)
	assert.Contains(t, fragment.Body.String(), `action="`+classPathFor(a, "?year="+strconv.Itoa(current)+"&amp;inactive=1")+`"`)
}

func TestClassRenameAndValidation(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	id := createClass(t, env, admin, current, "7А")
	createClass(t, env, admin, current, "7Б")

	renamed := servertest.PostForm(t, env.Handler, classPathFor(id, "?year="+strconv.Itoa(current)), url.Values{"name": {"7В"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, renamed, classesURL(current, ""))

	class, err := env.School.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "7В", class.Name)
	assert.Equal(t, current, class.Year)

	taken := servertest.PostForm(t, env.Handler, classPathFor(id, "?year="+strconv.Itoa(current)), url.Values{"name": {"7Б"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.Contains(t, taken.Body.String(), "Такой класс в этом году уже есть")
	assert.Equal(t, 1, strings.Count(taken.Body.String(), `value="7Б"`))
	assert.Contains(t, taken.Body.String(), ">7Б</span>")

	blank := servertest.PostForm(t, env.Handler, classPathFor(id, "?year="+strconv.Itoa(current)), url.Values{"name": {""}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "Укажите название")

	class, err = env.School.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "7В", class.Name)
}

func TestClassDeactivateHidesButKeepsName(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()
	year := strconv.Itoa(current)

	id := createClass(t, env, admin, current, "7А")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(id, "/deactivate?year="+year+"&inactive=1"), nil, []*http.Cookie{admin}, nil), classesURL(current, "&inactive=1"))

	visible := servertest.Get(t, env.Handler, classesURL(current, ""), admin).Body.String()
	assert.NotContains(t, visible, "7А")
	assert.Contains(t, visible, "Показать удалённые")

	all := servertest.Get(t, env.Handler, classesURL(current, "&inactive=1"), admin).Body.String()
	assert.Contains(t, all, "7А")
	assert.Contains(t, all, "удалён")
	assert.Contains(t, all, "Скрыть удалённые")
	assert.Contains(t, all, `href="`+classesURL(current, "")+`"`)
	assert.NotContains(t, all, `formaction="`+classPathFor(id, "/activate?year="+year+"&amp;inactive=1")+`"`)
	assert.Contains(t, all, `action="`+classesURL(current, "&amp;inactive=1")+`"`)

	restoring := servertest.Get(t, env.Handler, classesURL(current, "&inactive=1&edit="+strconv.FormatInt(id, 10)), admin).Body.String()
	assert.Contains(t, restoring, `formaction="`+classPathFor(id, "/activate?year="+year+"&amp;inactive=1")+`"`)
	assert.Contains(t, restoring, `aria-label="Восстановить"`)

	card := servertest.Get(t, env.Handler, classPathFor(id, ""), admin).Body.String()
	assert.Contains(t, card, "удалён")

	duplicate := servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.Contains(t, duplicate.Body.String(), "Такой класс в этом году уже есть")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(id, "/activate?year="+year), nil, []*http.Cookie{admin}, nil), classesURL(current, ""))

	class, err := env.School.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.True(t, class.Active)
}

func TestClassHtmxRequestsGetListFragment(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()
	htmx := map[string]string{"HX-Request": "true"}

	created := servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, created.Code)
	assert.NotContains(t, created.Body.String(), "<html")
	assert.Contains(t, created.Body.String(), `id="classes"`)
	assert.Contains(t, created.Body.String(), ">7А</span>")

	duplicate := servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.NotContains(t, duplicate.Body.String(), "<html")
	assert.Contains(t, duplicate.Body.String(), "Такой класс в этом году уже есть")
}

func TestClassUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/classes/999", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/classes/abc", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/classes/new", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/classes/999", url.Values{"name": {"X"}}, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/classes/999/deactivate", nil, []*http.Cookie{admin}, nil).Code)

	_, err := env.School.ClassByID(t.Context(), 999)
	assert.ErrorIs(t, err, school.ErrNotFound)
}

func TestClassesHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.LoginAs(t, "teacher")
	current := env.School.CurrentYear()

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/admin/classes"), "/login")
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/classes", teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, classesURL(current, ""), url.Values{"name": {"7А"}}, []*http.Cookie{teacher}, nil).Code)

	classes, err := env.School.Classes(t.Context(), current, true)
	require.NoError(t, err)
	assert.Empty(t, classes)
}

func TestClassWithStudentsCannotBeDeactivated(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()
	year := strconv.Itoa(current)

	id := createClass(t, env, admin, current, "7А")

	student, err := env.Auth.CreateUser(t.Context(), auth.NewUser{Role: auth.RoleStudent, Name: auth.Name{Last: "Козлов", First: "Пётр", Middle: "Ильич"}})
	require.NoError(t, err)
	require.NoError(t, env.School.SetStudentClass(t.Context(), student.User.ID, id))

	refused := servertest.PostForm(t, env.Handler, classPathFor(id, "/deactivate?year="+year), nil, []*http.Cookie{admin}, nil)
	require.Equal(t, http.StatusOK, refused.Code)
	assert.Contains(t, refused.Body.String(), "Сначала уберите учеников из класса")
	assert.Contains(t, refused.Body.String(), "<html")

	class, err := env.School.ClassByID(t.Context(), id)
	require.NoError(t, err)
	assert.True(t, class.Active)

	fragment := servertest.PostForm(t, env.Handler, classPathFor(id, "/deactivate?year="+year), nil, []*http.Cookie{admin},
		map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), "Сначала уберите учеников из класса")
	assert.NotContains(t, fragment.Body.String(), "<html")

	require.NoError(t, env.School.RemoveClassStudent(t.Context(), id, student.User.ID))
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(id, "/deactivate?year="+year), nil, []*http.Cookie{admin}, nil), classesURL(current, ""))
}

func TestClassesSortedNaturallyWithStudentCounts(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	for _, name := range []string{"10А", "2Б", "1А", "11Б", "Подготовительный"} {
		createClass(t, env, admin, current, name)
	}

	classes, err := env.School.Classes(t.Context(), current, false)
	require.NoError(t, err)

	names := make([]string, 0, len(classes))
	for _, class := range classes {
		names = append(names, class.Name)
	}

	assert.Equal(t, []string{"Подготовительный", "1А", "2Б", "10А", "11Б"}, names)

	env.CreateUser(t, auth.RoleStudent, "ivanov", "Иванов Пётр")
	env.CreateUser(t, auth.RoleStudent, "petrova", "Петрова Анна")
	require.NoError(t, env.School.AddClassStudent(t.Context(), classes[1].ID, userIDByLogin(t, env, "ivanov")))
	require.NoError(t, env.School.AddClassStudent(t.Context(), classes[1].ID, userIDByLogin(t, env, "petrova")))

	body := servertest.Get(t, env.Handler, "/admin/classes", admin).Body.String()
	assert.Contains(t, body, ">2 ученика<")
	assert.Contains(t, body, ">0 учеников<")
	assert.Less(t, strings.Index(body, ">2Б</span>"), strings.Index(body, ">10А</span>"))
}

func TestClassesNextYearTabIsEmptyAndReadOnly(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	next := env.School.CurrentYear() + 1

	body := servertest.Get(t, env.Handler, classesURL(next, ""), admin).Body.String()
	assert.Contains(t, body, `href="`+classesURL(next, "")+`" aria-current="page"`)
	assert.Contains(t, body, "В "+school.YearName(next)+" пока нет классов")
	assert.NotContains(t, body, `action="`+classesURL(next, "")+`"`)
	assert.NotContains(t, body, `href="`+transferURL+`"`)
}
