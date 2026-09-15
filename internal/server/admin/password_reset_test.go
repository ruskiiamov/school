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
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestPasswordResetSearchShowsActiveUsersOfAllRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	env.CreateUser(t, auth.RoleTeacher, "smirnova.t", "Смирнова Мария")
	env.CreateUser(t, auth.RoleParent, "smirnov.p", "Смирнов Пётр")
	env.CreateUser(t, auth.RoleStudent, "smirnova.s", "Смирнова Анна")
	env.CreateUser(t, auth.RoleStudent, "smirnov.gone", "Смирнов Удалённый")
	env.CreateUser(t, auth.RoleTeacher, "kuznetsov.o", "Кузнецов Олег")

	classID, err := env.School.CreateClass(t.Context(), "7А")
	require.NoError(t, err)

	studentID := userIDByLogin(t, env, "smirnova.s")
	require.NoError(t, env.School.SetStudentClass(t.Context(), studentID, classID))
	require.NoError(t, env.Auth.SetUserActive(t.Context(), userIDByLogin(t, env, "smirnov.gone"), false))

	empty := servertest.Get(t, env.Handler, "/admin/password-reset", admin)
	require.Equal(t, http.StatusOK, empty.Code)
	assert.Contains(t, empty.Body.String(), "Введите ФИО, чтобы найти пользователя")
	assert.NotContains(t, empty.Body.String(), "Смирнова")
	assert.NotContains(t, empty.Body.String(), "Кузнецов")
	assert.Contains(t, empty.Body.String(), `aria-current="page"`)

	found := servertest.Get(t, env.Handler, "/admin/password-reset?q=смирн", admin).Body.String()
	assert.Contains(t, found, "Смирнова Мария")
	assert.Contains(t, found, "Смирнов Пётр")
	assert.Contains(t, found, "Смирнова Анна")
	assert.NotContains(t, found, "Смирнов Удалённый")
	assert.NotContains(t, found, "Кузнецов Олег")
	assert.Contains(t, found, ">Учитель<")
	assert.Contains(t, found, ">Ученик<")
	assert.Contains(t, found, ">Родитель<")
	assert.Contains(t, found, ">7А<")
	assert.Contains(t, found, ">smirnova.t<")
	assert.Contains(t, found, `action="/admin/password-reset/`+strconv.FormatInt(studentID, 10)+`?q=%D1%81%D0%BC%D0%B8%D1%80%D0%BD"`)
	assert.Contains(t, found, `hx-confirm="Сбросить пароль пользователю Смирнова Анна?"`)
	assert.Contains(t, found, `data-confirm-ok="Сбросить"`)
	assert.Contains(t, found, ">Сбросить пароль<")
	assert.Less(t, strings.Index(found, "Смирнов Пётр"), strings.Index(found, "Смирнова Анна"))
	assert.Less(t, strings.Index(found, "Смирнова Анна"), strings.Index(found, "Смирнова Мария"))

	none := servertest.Get(t, env.Handler, "/admin/password-reset?q=иванов", admin).Body.String()
	assert.Contains(t, none, "Ничего не найдено")

	fragment := servertest.Get(t, env.Handler, "/admin/password-reset?q=олег", admin, map[string]string{"HX-Request": "true"}).Body.String()
	assert.Contains(t, fragment, "Кузнецов Олег")
	assert.NotContains(t, fragment, "<html")
}

func TestPasswordResetShowsPasswordOnceAndDropsSessions(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	env.CreateUser(t, auth.RoleTeacher, "smirnova.m", "Смирнова Мария")
	teacher := env.LoginAs(t, "smirnova.m")

	id := userIDByLogin(t, env, "smirnova.m")

	path := "/admin/password-reset/" + strconv.FormatInt(id, 10)
	createdPath := path + "/created?q=%D1%81%D0%BC%D0%B8"

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, path+"?q=сми", nil, []*http.Cookie{admin}, nil), createdPath)

	changed := servertest.Get(t, env.Handler, createdPath, admin).Body.String()
	assert.Contains(t, changed, "Пароль изменён")
	assert.Contains(t, changed, "Смирнова Мария")
	assert.Contains(t, changed, ">smirnova.m<")
	assert.Contains(t, changed, "передайте его учителю")
	assert.Contains(t, changed, `href="/admin/password-reset?q=%D1%81%D0%BC%D0%B8"`)
	assert.Contains(t, changed, ">К поиску<")
	assert.NotContains(t, changed, "Ещё учителя")

	values := credentialValue.FindAllStringSubmatch(changed, -1)
	require.Len(t, values, 2, changed)
	assert.Regexp(t, "^[A-Za-z0-9]{10}$", values[1][1])

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, createdPath, admin), "/admin/password-reset?q=%D1%81%D0%BC%D0%B8")

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/journal", teacher), "/login")

	_, err := env.Auth.Login(t.Context(), "smirnova.m", "smirnova.m-password")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)

	servertest.LoginWith(t, env.Handler, "smirnova.m", values[1][1])

	htmx := servertest.PostForm(t, env.Handler, path, nil, []*http.Cookie{admin}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusNoContent, htmx.Code)
	assert.Equal(t, path+"/created", htmx.Header().Get("HX-Redirect"))

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/admin/teachers/"+strconv.FormatInt(id, 10)+"/created", admin), "/admin/teachers")

	again := servertest.Get(t, env.Handler, path+"/created", admin)
	assert.Equal(t, http.StatusOK, again.Code)
	assert.Contains(t, again.Body.String(), "Пароль изменён")
	assert.Contains(t, again.Body.String(), `href="/admin/password-reset"`)
}

func TestPasswordResetRejectsAdminUnknownAndForeignRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	env.CreateUser(t, auth.RoleTeacher, "smirnova.m", "Смирнова Мария")
	teacher := env.LoginAs(t, "smirnova.m")

	adminID := userIDByLogin(t, env, servertest.AdminLogin)

	for _, path := range []string{
		"/admin/password-reset/" + strconv.FormatInt(adminID, 10),
		"/admin/password-reset/999",
		"/admin/password-reset/abc",
	} {
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, path, url.Values{}, []*http.Cookie{admin}, nil).Code, path)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, path+"/created", admin).Code, path)
	}

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/password-reset", teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/password-reset/"+strconv.FormatInt(adminID, 10), nil, []*http.Cookie{teacher}, nil).Code)
	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/admin/password-reset"), "/login")
}

func userIDByLogin(t *testing.T, env *servertest.Env, login string) int64 {
	t.Helper()

	var id int64
	require.NoError(t, env.DB.QueryRowContext(t.Context(), "SELECT id FROM users WHERE login = ?", login).Scan(&id))

	return id
}
