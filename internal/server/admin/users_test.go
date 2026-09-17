package admin_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

var createdLocation = regexp.MustCompile(`^(/admin/[a-z]+)/(\d+)/created$`)

func createUserVia(t *testing.T, env *servertest.Env, admin *http.Cookie, path string, form url.Values) (int64, string) {
	t.Helper()

	recorder := servertest.PostForm(t, env.Handler, path, form, []*http.Cookie{admin}, nil)
	require.Equal(t, http.StatusSeeOther, recorder.Code, recorder.Body.String())

	match := createdLocation.FindStringSubmatch(recorder.Header().Get("Location"))
	require.NotNil(t, match, recorder.Header().Get("Location"))

	id, err := strconv.ParseInt(match[2], 10, 64)
	require.NoError(t, err)

	return id, match[0]
}

var credentialValue = regexp.MustCompile(`<dd class="font-mono text-slate-900">([^<]+)</dd>`)

func takeCredentials(t *testing.T, env *servertest.Env, admin *http.Cookie, createdPath string) (string, string) {
	t.Helper()

	recorder := servertest.Get(t, env.Handler, createdPath, admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	values := credentialValue.FindAllStringSubmatch(recorder.Body.String(), -1)
	require.Len(t, values, 2, recorder.Body.String())

	return values[0][1], values[1][1]
}

func userPathFor(path string, id int64, suffix string) string {
	return path + "/" + strconv.FormatInt(id, 10) + suffix
}

func TestUserSectionsListAndCreate(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	sections := []struct {
		path, empty, created string
		role                 auth.Role
	}{
		{"/admin/teachers", "Пока нет учителей", "Учитель создан", auth.RoleTeacher},
		{"/admin/students", "Пока нет учеников", "Ученик создан", auth.RoleStudent},
		{"/admin/parents", "Пока нет родителей", "Родитель создан", auth.RoleParent},
	}

	for _, section := range sections {
		empty := servertest.Get(t, env.Handler, section.path, admin)
		require.Equal(t, http.StatusOK, empty.Code)
		assert.Contains(t, empty.Body.String(), section.empty)
		assert.Contains(t, empty.Body.String(), `action="`+section.path+`"`)
		assert.Contains(t, empty.Body.String(), "ФИО нового пользователя")
		assert.NotContains(t, empty.Body.String(), `name="login"`)
		assert.NotContains(t, empty.Body.String(), `name="password"`)
		assert.Contains(t, empty.Body.String(), `id="users-search" hx-preserve`)
		assert.Contains(t, empty.Body.String(), `href="`+section.path+`?inactive=1"`)
		assert.NotContains(t, empty.Body.String(), "required")

		id, createdPath := createUserVia(t, env, admin, section.path, url.Values{"full_name": {"Смирнова Мария Петровна"}})

		user, err := env.Auth.UserByID(t.Context(), id)
		require.NoError(t, err)
		assert.Equal(t, section.role, user.Role)
		assert.True(t, user.Active)

		created := servertest.Get(t, env.Handler, createdPath, admin)
		require.Equal(t, http.StatusOK, created.Code)
		assert.Contains(t, created.Body.String(), section.created)
		assert.Contains(t, created.Body.String(), "Смирнова Мария Петровна")
		assert.Contains(t, created.Body.String(), ">"+user.Login+"<")
		assert.Contains(t, created.Body.String(), "показан один раз")
		assert.Contains(t, created.Body.String(), `href="`+section.path+`"`)

		servertest.AssertRedirect(t, servertest.Get(t, env.Handler, createdPath, admin), section.path)

		list := servertest.Get(t, env.Handler, section.path, admin).Body.String()
		assert.Contains(t, list, "Смирнова Мария Петровна")
		assert.Contains(t, list, user.Login)
		assert.Contains(t, list, `href="`+section.path+`?edit=`+strconv.FormatInt(id, 10)+`"`)
		assert.NotContains(t, list, `formaction="`+userPathFor(section.path, id, "/deactivate")+`"`)
		assert.NotContains(t, list, section.empty)

		editing := servertest.Get(t, env.Handler, section.path+"?edit="+strconv.FormatInt(id, 10), admin).Body.String()
		assert.Contains(t, editing, `formaction="`+userPathFor(section.path, id, "/deactivate")+`"`)
		assert.Contains(t, editing, "data-edit-form")
	}

	teachers, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: auth.RoleTeacher})
	require.NoError(t, err)
	require.Len(t, teachers, 1)
	assert.Equal(t, "smirnova.m", teachers[0].Login)

	students, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: auth.RoleStudent})
	require.NoError(t, err)
	require.Len(t, students, 1)
	assert.Equal(t, "smirnova.m2", students[0].Login)
}

func TestUserCreateValidationKeepsInput(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	blank := servertest.PostForm(t, env.Handler, "/admin/teachers", url.Values{"full_name": {" "}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "<html")
	assert.Contains(t, blank.Body.String(), "Укажите ФИО")

	htmx := map[string]string{"HX-Request": "true"}

	long := servertest.PostForm(t, env.Handler, "/admin/parents", url.Values{"full_name": {strings.Repeat("Я", 101)}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, long.Code)
	assert.NotContains(t, long.Body.String(), "<html")
	assert.Contains(t, long.Body.String(), `id="users"`)
	assert.Contains(t, long.Body.String(), "ФИО длиннее 100 символов")
	assert.Contains(t, long.Body.String(), `value="`+strings.Repeat("Я", 101)+`"`)

	users, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: auth.RoleParent, IncludeInactive: true})
	require.NoError(t, err)
	assert.Empty(t, users)

	created := servertest.PostForm(t, env.Handler, "/admin/parents", url.Values{"full_name": {"Петров Пётр"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusNoContent, created.Code)
	assert.Regexp(t, createdLocation, created.Header().Get("HX-Redirect"))
}

func TestUserCreatedPageShowsPasswordOnce(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	_, createdPath := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Сидорова Анна"}})

	other := servertest.Login(t, env.Handler)
	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, createdPath, other), "/admin/teachers")

	created := servertest.Get(t, env.Handler, createdPath, admin)
	require.Equal(t, http.StatusOK, created.Code)
	assert.Contains(t, created.Body.String(), ">sidorova.a<")
	assert.Contains(t, created.Body.String(), "Ещё учителя")

	values := credentialValue.FindAllStringSubmatch(created.Body.String(), -1)
	require.Len(t, values, 2)
	assert.Equal(t, "sidorova.a", values[0][1])
	assert.Regexp(t, "^[A-Za-z0-9]{10}$", values[1][1])

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, createdPath, admin), "/admin/teachers")

	servertest.LoginWith(t, env.Handler, "sidorova.a", values[1][1])
}

func TestStudentClassInFormListAndFilter(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	noClasses := servertest.Get(t, env.Handler, "/admin/students", admin).Body.String()
	assert.NotContains(t, noClasses, "Без класса")
	assert.NotContains(t, noClasses, "Все классы")

	classA := createClass(t, env, admin, current, "7А")
	classB := createClass(t, env, admin, current, "7Б")
	past := createPastClass(t, env, current-1, "6А")
	a := strconv.FormatInt(classA, 10)
	b := strconv.FormatInt(classB, 10)

	form := servertest.Get(t, env.Handler, "/admin/students", admin).Body.String()
	assert.Contains(t, form, `value="0" selected>Без класса<`)
	assert.Contains(t, form, `value="`+a+`">7А<`)
	assert.NotContains(t, form, "6А")

	forged := servertest.PostForm(t, env.Handler, "/admin/students", url.Values{"full_name": {"Смирнова Мария"}, "class": {strconv.FormatInt(past, 10)}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, forged.Code)
	assert.Contains(t, forged.Body.String(), "Такого класса нет в текущем году")
	assert.Contains(t, forged.Body.String(), `value="Смирнова Мария"`)

	none, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	require.NoError(t, err)
	assert.Empty(t, none)

	inA, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Смирнова Мария"}, "class": {a}})
	inB, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Петров Иван"}, "class": {b}})
	free, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Кузнецов Олег"}})

	class, found, err := env.School.StudentClass(t.Context(), inA)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, classA, class.ID)

	_, found, err = env.School.StudentClass(t.Context(), free)
	require.NoError(t, err)
	assert.False(t, found)

	all := servertest.Get(t, env.Handler, "/admin/students", admin).Body.String()
	assert.Contains(t, all, "Смирнова Мария")
	assert.Contains(t, all, "Петров Иван")
	assert.Contains(t, all, "без класса")
	assert.Contains(t, all, ">7А<")

	filtered := servertest.Get(t, env.Handler, "/admin/students?class="+b, admin).Body.String()
	assert.Contains(t, filtered, "Петров Иван")
	assert.NotContains(t, filtered, "Смирнова Мария")
	assert.NotContains(t, filtered, "Кузнецов Олег")
	assert.Contains(t, filtered, `value="`+b+`" selected>7Б<`)
	assert.Contains(t, filtered, `href="/admin/students?class=`+b+`&amp;edit=`+strconv.FormatInt(inB, 10)+`"`)
	assert.Contains(t, filtered, `href="/admin/students?class=`+b+`&amp;inactive=1"`)

	edit := servertest.Get(t, env.Handler, "/admin/students?edit="+strconv.FormatInt(inA, 10), admin).Body.String()
	assert.Contains(t, edit, `value="`+a+`" selected>7А<`)
	assert.Contains(t, edit, `value="Смирнова Мария"`)
	assert.Contains(t, edit, `action="`+userPathFor("/admin/students", inA, "")+`"`)
	assert.NotContains(t, edit, "Сменить пароль")
	row := edit[strings.Index(edit, `action="`+userPathFor("/admin/students", inA, "")+`"`):]
	assert.Less(t, strings.Index(row, `name="full_name"`), strings.Index(row, `name="class"`))
	assert.Less(t, strings.Index(row, `name="class"`), strings.Index(row, `name="login"`))
	assert.Contains(t, edit, `data-cancel="/admin/students"`)
	assert.NotContains(t, edit, `aria-label="Отмена"`)
	assert.NotContains(t, edit, `value="Петров Иван"`)
	assert.Equal(t, 1, strings.Count(edit, "ФИО нового пользователя"))

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, userPathFor("/admin/students", inA, ""), url.Values{"full_name": {"Смирнова Мария"}, "login": {"smirnova.m"}, "class": {b}}, []*http.Cookie{admin}, nil), "/admin/students")

	class, found, err = env.School.StudentClass(t.Context(), inA)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, classB, class.ID)

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, userPathFor("/admin/students", inA, "?class="+b), url.Values{"full_name": {"Смирнова Мария"}, "login": {"smirnova.m"}, "class": {"0"}}, []*http.Cookie{admin}, nil), "/admin/students?class="+b)

	_, found, err = env.School.StudentClass(t.Context(), inA)
	require.NoError(t, err)
	assert.False(t, found)

	classes, err := env.School.StudentClasses(t.Context(), current)
	require.NoError(t, err)
	assert.Len(t, classes, 1)
	assert.Equal(t, "7Б", classes[inB].Name)
}

func TestUserSearchAndInactiveToggle(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	id, _ := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Смирнова Мария"}})
	createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Сидорова Анна"}})

	const query = "%D0%BC%D0%98%D0%A0%D0%BD"

	found := servertest.Get(t, env.Handler, "/admin/teachers?q="+query, admin).Body.String()
	assert.Contains(t, found, "Смирнова Мария")
	assert.NotContains(t, found, "Сидорова Анна")
	assert.Contains(t, found, `href="/admin/teachers?q=`+query+`&amp;edit=`+strconv.FormatInt(id, 10)+`"`)
	assert.Contains(t, found, `href="/admin/teachers?inactive=1&amp;q=`+query+`"`)
	assert.Contains(t, found, `action="/admin/teachers?q=`+query+`"`)

	nothing := servertest.Get(t, env.Handler, "/admin/teachers?q=zzz", admin).Body.String()
	assert.Contains(t, nothing, "Ничего не найдено")
	assert.NotContains(t, nothing, "Пока нет учителей")

	fragment := servertest.Get(t, env.Handler, "/admin/teachers?q=zzz", admin, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.NotContains(t, fragment.Body.String(), "<html")
	assert.Contains(t, fragment.Body.String(), `id="users"`)
	assert.Contains(t, fragment.Body.String(), `href="/admin/teachers?inactive=1&amp;q=zzz"`)

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, userPathFor("/admin/teachers", id, "/deactivate?inactive=1"), nil, []*http.Cookie{admin}, nil), "/admin/teachers?inactive=1")

	visible := servertest.Get(t, env.Handler, "/admin/teachers", admin).Body.String()
	assert.NotContains(t, visible, "Смирнова Мария")
	assert.Contains(t, visible, "Показать удалённые")

	all := servertest.Get(t, env.Handler, "/admin/teachers?inactive=1", admin).Body.String()
	assert.Contains(t, all, "Смирнова Мария")
	assert.Contains(t, all, "удалён")
	assert.Contains(t, all, "Скрыть удалённые")
	assert.Contains(t, all, `name="inactive" value="1"`)
	assert.NotContains(t, all, `formaction="`+userPathFor("/admin/teachers", id, "/activate?inactive=1")+`"`)

	restoring := servertest.Get(t, env.Handler, "/admin/teachers?inactive=1&edit="+strconv.FormatInt(id, 10), admin).Body.String()
	assert.Contains(t, restoring, `formaction="`+userPathFor("/admin/teachers", id, "/activate?inactive=1")+`"`)
}

func TestUserDeactivateAndActivate(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	htmx := map[string]string{"HX-Request": "true"}

	id, createdPath := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Смирнова Мария"}})
	_, password := takeCredentials(t, env, admin, createdPath)
	teacher := servertest.LoginWith(t, env.Handler, "smirnova.m", password)
	assert.Equal(t, http.StatusOK, servertest.Get(t, env.Handler, "/journal", teacher).Code)

	deactivated := servertest.PostForm(t, env.Handler, userPathFor("/admin/teachers", id, "/deactivate"), nil, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, deactivated.Code)
	assert.NotContains(t, deactivated.Body.String(), "<html")
	assert.Contains(t, deactivated.Body.String(), "Пока нет учителей")

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/journal", teacher), "/login")

	denied := servertest.PostForm(t, env.Handler, "/login", url.Values{"login": {"smirnova.m"}, "password": {password}}, nil, nil)
	assert.Equal(t, http.StatusOK, denied.Code)
	assert.Contains(t, denied.Body.String(), "Неверный логин или пароль")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, userPathFor("/admin/teachers", id, "/activate"), nil, []*http.Cookie{admin}, nil), "/admin/teachers")
	servertest.LoginWith(t, env.Handler, "smirnova.m", password)
}

func TestUserUpdateValidationAndRename(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	id, _ := createUserVia(t, env, admin, "/admin/parents", url.Values{"full_name": {"Смирнова Мария"}})
	createUserVia(t, env, admin, "/admin/parents", url.Values{"full_name": {"Сидорова Анна"}})

	path := userPathFor("/admin/parents", id, "")

	taken := servertest.PostForm(t, env.Handler, path, url.Values{"full_name": {"Смирнова Мария"}, "login": {"sidorova.a"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.Contains(t, taken.Body.String(), "Такой логин уже есть")
	assert.Contains(t, taken.Body.String(), `action="`+path+`"`)
	assert.Equal(t, 1, strings.Count(taken.Body.String(), `value="sidorova.a"`))
	assert.Contains(t, taken.Body.String(), ">Сидорова Анна</span>")

	blank := servertest.PostForm(t, env.Handler, path, url.Values{"full_name": {""}, "login": {""}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "Укажите ФИО")
	assert.Contains(t, blank.Body.String(), "Укажите логин")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, path, url.Values{"full_name": {"Смирнова  Мария Петровна"}, "login": {"Maria"}}, []*http.Cookie{admin}, nil), "/admin/parents")

	user, err := env.Auth.UserByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "Смирнова Мария Петровна", user.FullName)
	assert.Equal(t, "maria", user.Login)
}

func TestUserSectionRejectsForeignRoleAndUnknownID(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	student, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Смирнова Мария"}})

	var adminID int64
	require.NoError(t, env.DB.QueryRowContext(t.Context(), "SELECT id FROM users WHERE login = ?", servertest.AdminLogin).Scan(&adminID))

	for _, path := range []string{
		userPathFor("/admin/teachers", student, "/created"),
		userPathFor("/admin/students", adminID, "/created"),
		"/admin/teachers/999/created",
		"/admin/teachers/abc/created",
	} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, path, admin).Code, path)
	}

	for _, path := range []string{
		userPathFor("/admin/teachers", student, "/deactivate"),
		userPathFor("/admin/students", adminID, "/deactivate"),
		userPathFor("/admin/teachers", adminID, ""),
		"/admin/teachers/999",
	} {
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, path, url.Values{"full_name": {"X"}, "login": {"x"}}, []*http.Cookie{admin}, nil).Code, path)
	}

	foreign := servertest.Get(t, env.Handler, "/admin/teachers?edit="+strconv.FormatInt(student, 10), admin).Body.String()
	assert.NotContains(t, foreign, "Смирнова Мария")

	user, err := env.Auth.UserByID(t.Context(), student)
	require.NoError(t, err)
	assert.True(t, user.Active)

	servertest.Login(t, env.Handler)
}

func TestUserSectionsHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	env.CreateUser(t, auth.RoleStudent, "student", "Смирнова Мария Петровна")
	teacher := env.LoginAs(t, "teacher")
	student := env.LoginAs(t, "student")

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/admin/students"), "/login")

	for _, path := range []string{"/admin/teachers", "/admin/students", "/admin/parents"} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, path, teacher).Code, path)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, path, student).Code, path)
	}

	forged := servertest.PostForm(t, env.Handler, "/admin/teachers", url.Values{"full_name": {"Кто-то"}}, []*http.Cookie{student}, nil)
	assert.Equal(t, http.StatusNotFound, forged.Code)

	teachers, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: auth.RoleTeacher, IncludeInactive: true})
	require.NoError(t, err)
	require.Len(t, teachers, 1)
	assert.Equal(t, "teacher", teachers[0].Login)
}
