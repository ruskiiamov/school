package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
)

func TestAccountPasswordPageAndSidebarLink(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.createUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	student := env.loginAs(t, "student")

	home := get(t, env.handler, "/", student).Body.String()
	assert.Contains(t, home, `href="/account/password"`)
	assert.Contains(t, home, "Сменить пароль")

	page := get(t, env.handler, "/account/password", student)
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

	env := newTestEnv(t)
	env.createUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	current := env.loginAs(t, "teacher")
	other := env.loginAs(t, "teacher")

	form := url.Values{
		"current": {"teacher-password"},
		"new":     {"новый пароль"},
		"repeat":  {"новый пароль"},
	}

	recorder := postForm(t, env.handler, "/account/password", form, []*http.Cookie{current}, nil)
	assertRedirect(t, recorder, "/account/password?done=1")

	done := get(t, env.handler, "/account/password?done=1", current)
	require.Equal(t, http.StatusOK, done.Code)
	assert.Contains(t, done.Body.String(), "Пароль изменён")

	assertRedirect(t, get(t, env.handler, "/", other), "/login")

	assert.Equal(t, http.StatusOK, get(t, env.handler, "/", current).Code)
	loginWith(t, env.handler, "teacher", "новый пароль")
}

func TestAccountPasswordChangeViaHTMXRedirects(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.createUser(t, auth.RoleParent, "parent", "Петрова Ольга Николаевна")
	parent := env.loginAs(t, "parent")

	form := url.Values{
		"current": {"parent-password"},
		"new":     {"long-enough"},
		"repeat":  {"long-enough"},
	}

	recorder := postForm(t, env.handler, "/account/password", form, []*http.Cookie{parent},
		map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "/account/password?done=1", recorder.Header().Get("HX-Redirect"))
	loginWith(t, env.handler, "parent", "long-enough")
}

func TestAccountPasswordErrorsViaHTMXKeepFields(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.createUser(t, auth.RoleParent, "parent", "Петрова Ольга Николаевна")
	parent := env.loginAs(t, "parent")

	form := url.Values{"current": {"wrong"}, "new": {"long-enough"}, "repeat": {"long-enough"}}

	recorder := postForm(t, env.handler, "/account/password", form, []*http.Cookie{parent},
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

	env := newTestEnv(t)
	env.createUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	student := env.loginAs(t, "student")

	form := url.Values{
		"current": {"wrong"},
		"new":     {"short"},
		"repeat":  {"other"},
	}

	recorder := postForm(t, env.handler, "/account/password", form, []*http.Cookie{student}, nil)
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

	assert.Equal(t, http.StatusOK, get(t, env.handler, "/", student).Code)
	loginWith(t, env.handler, "student", "student-password")
}

func TestAccountPasswordIsHiddenFromAdminAndAnonymous(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)

	assertRedirect(t, get(t, env.handler, "/account/password"), "/login")

	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/account/password", admin).Code)
	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, "/account/password", nil, []*http.Cookie{admin}, nil).Code)

	home := get(t, env.handler, "/", admin).Body.String()
	assert.NotContains(t, home, `href="/account/password"`)
	assert.Contains(t, home, "Иванова Мария Петровна")
	assert.Contains(t, home, "Администратор")
}
