package account_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestAccountPasswordPageAndSidebarLink(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	student := env.LoginAs(t, "student")

	home := servertest.Get(t, env.Handler, "/", student).Body.String()
	assert.Contains(t, home, `href="/account/password"`)
	assert.Contains(t, home, "Сменить пароль")

	page := servertest.Get(t, env.Handler, "/account/password", student)
	assert.Equal(t, http.StatusOK, page.Code)
	body := page.Body.String()
	assert.Contains(t, body, "Смена пароля")
	assert.Contains(t, body, `action="/account/password"`)
	assert.Contains(t, body, `hx-target="#password-status"`)
	assert.Contains(t, body, `id="error-new"`)
	assert.Contains(t, body, `autocomplete="current-password"`)
	assert.Contains(t, body, `autocomplete="new-password"`)
	assert.Contains(t, body, `aria-current="page"`)
	assert.NotContains(t, body, "required")
	assert.Contains(t, body, `name="current" value=""`)
	assert.Equal(t, 3, strings.Count(body, `data-toggle-password`))
}

func TestAccountPasswordChangeKeepsCurrentSession(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	current := env.LoginAs(t, "teacher")
	other := env.LoginAs(t, "teacher")

	form := url.Values{
		"current": {"teacher-password"},
		"new":     {"новый пароль"},
		"repeat":  {"новый пароль"},
	}

	recorder := servertest.PostForm(t, env.Handler, "/account/password", form, []*http.Cookie{current}, nil)
	servertest.AssertRedirect(t, recorder, "/account/password?done=1")

	done := servertest.Get(t, env.Handler, "/account/password?done=1", current)
	require.Equal(t, http.StatusOK, done.Code)
	assert.Contains(t, done.Body.String(), "Пароль изменён")

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/", other), "/login")

	assert.Equal(t, http.StatusOK, servertest.Get(t, env.Handler, "/", current).Code)
	servertest.LoginWith(t, env.Handler, "teacher", "новый пароль")
}

func TestAccountPasswordChangeViaHTMXRedirects(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleParent, "parent", "Петрова Ольга Николаевна")
	parent := env.LoginAs(t, "parent")

	form := url.Values{
		"current": {"parent-password"},
		"new":     {"long-enough"},
		"repeat":  {"long-enough"},
	}

	recorder := servertest.PostForm(t, env.Handler, "/account/password", form, []*http.Cookie{parent},
		map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "/account/password?done=1", recorder.Header().Get("HX-Redirect"))
	servertest.LoginWith(t, env.Handler, "parent", "long-enough")
}

func TestAccountPasswordErrorsViaHTMXKeepFields(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleParent, "parent", "Петрова Ольга Николаевна")
	parent := env.LoginAs(t, "parent")

	form := url.Values{"current": {"wrong"}, "new": {"long-enough"}, "repeat": {"long-enough"}}

	recorder := servertest.PostForm(t, env.Handler, "/account/password", form, []*http.Cookie{parent},
		map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, `id="error-current" data-error hx-swap-oob="true"`)
	assert.Contains(t, body, "Неверный текущий пароль")
	assert.Contains(t, body, `id="error-new" data-error hx-swap-oob="true" class="text-sm text-rose-600"></span>`)
	assert.NotContains(t, body, "<form")
	assert.NotContains(t, body, `type="password"`)
	assert.NotContains(t, body, "<html")
}

func TestAccountPasswordChangeShowsErrors(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	student := env.LoginAs(t, "student")

	form := url.Values{
		"current": {"wrong"},
		"new":     {"short"},
		"repeat":  {"other"},
	}

	recorder := servertest.PostForm(t, env.Handler, "/account/password", form, []*http.Cookie{student}, nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "<html")
	assert.Contains(t, body, "Неверный текущий пароль")
	assert.Contains(t, body, "Пароль короче 8 символов")
	assert.Contains(t, body, "Пароли не совпадают")
	assert.NotContains(t, body, "Пароль изменён")
	assert.Contains(t, body, `name="current" value="wrong"`)
	assert.Contains(t, body, `name="new" value="short"`)
	assert.Contains(t, body, `name="repeat" value="other"`)

	assert.Equal(t, http.StatusOK, servertest.Get(t, env.Handler, "/", student).Code)
	servertest.LoginWith(t, env.Handler, "student", "student-password")
}

func TestAccountPasswordIsHiddenFromAdminAndAnonymous(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/account/password"), "/login")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/account/password", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/account/password", nil, []*http.Cookie{admin}, nil).Code)

	home := servertest.Get(t, env.Handler, "/", admin).Body.String()
	assert.NotContains(t, home, `href="/account/password"`)
	assert.Contains(t, home, "Иванова М. П.")
	assert.Contains(t, home, "Администратор")
}
