package account_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestLoginPageRenders(t *testing.T) {
	t.Parallel()

	recorder := servertest.Get(t, servertest.New(t).Handler, "/login")
	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, body, `lang="ru"`)
	assert.Contains(t, body, `name="viewport"`)
	assert.Contains(t, body, ">Школа</h1>")
	assert.Contains(t, body, "Школа №1")
	assert.Contains(t, body, `autocomplete="current-password"`)
	assert.Contains(t, body, `data-toggle-password`)
	assert.Contains(t, body, `aria-label="Показать пароль"`)
	assert.Contains(t, body, `method="post"`)
	assert.Contains(t, body, `action="/login"`)
}

func TestLoginSuccessSetsSecureCookie(t *testing.T) {
	t.Parallel()

	cookie := servertest.Login(t, servertest.New(t).Handler)

	assert.Equal(t, "sid", cookie.Name)
	assert.NotEmpty(t, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, "/", cookie.Path)
	assert.Positive(t, cookie.MaxAge)
}

func TestLoginFailureReturnsFragmentWithoutCookie(t *testing.T) {
	t.Parallel()

	recorder := servertest.PostForm(t, servertest.New(t).Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {"wrong"}},
		nil, map[string]string{"HX-Request": "true"})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, servertest.Cookies(t, recorder))
	assert.Contains(t, recorder.Body.String(), "Неверный логин или пароль")
	assert.NotContains(t, recorder.Body.String(), "<html")
}

func TestLoginFailureWithoutHtmxRendersFullPage(t *testing.T) {
	t.Parallel()

	recorder := servertest.PostForm(t, servertest.New(t).Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {"wrong"}}, nil, nil)

	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "<html")
	assert.Contains(t, body, "Неверный логин или пароль")
	assert.Contains(t, body, `value="admin"`)
}

func TestLoginPageRedirectsAuthenticatedUser(t *testing.T) {
	t.Parallel()

	handler := servertest.New(t).Handler
	recorder := servertest.Get(t, handler, "/login", servertest.Login(t, handler))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Location"))
}

func TestLogoutClearsCookieAndSession(t *testing.T) {
	t.Parallel()

	handler := servertest.New(t).Handler
	cookie := servertest.Login(t, handler)

	recorder := servertest.PostForm(t, handler, "/logout", nil, []*http.Cookie{cookie},
		map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("HX-Redirect"))

	cleared := servertest.Cookies(t, recorder)
	require.Len(t, cleared, 1)
	assert.Empty(t, cleared[0].Value)
	assert.Negative(t, cleared[0].MaxAge)

	after := servertest.Get(t, handler, "/", cookie)
	assert.Equal(t, http.StatusSeeOther, after.Code)
	assert.Equal(t, "/login", after.Header().Get("Location"))
}
