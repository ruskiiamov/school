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

func TestLoginBlockedAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	wrong := url.Values{"login": {servertest.AdminLogin}, "password": {"wrong"}}

	for range 5 {
		recorder := servertest.PostForm(t, env.Handler, "/login", wrong, nil, nil)
		require.Contains(t, recorder.Body.String(), "Неверный логин или пароль")
	}

	recorder := servertest.PostForm(t, env.Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}}, nil, nil)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, servertest.Cookies(t, recorder))
	assert.Contains(t, recorder.Body.String(), "Слишком много неудачных попыток")
}

func TestLoginRemembersDevice(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	correct := url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}}

	first := servertest.PostForm(t, env.Handler, "/login", correct, nil, nil)
	device := servertest.CookieNamed(t, first, servertest.DeviceCookie)

	assert.NotEmpty(t, device.Value)
	assert.True(t, device.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, device.SameSite)
	assert.Equal(t, "/login", device.Path)
	assert.Positive(t, device.MaxAge)

	second := servertest.PostForm(t, env.Handler, "/login", correct, []*http.Cookie{device}, nil)
	assert.Equal(t, device.Value, servertest.CookieNamed(t, second, servertest.DeviceCookie).Value)
}

func TestLoginFromKnownDeviceSurvivesLockout(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	correct := url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}}
	wrong := url.Values{"login": {servertest.AdminLogin}, "password": {"wrong"}}

	device := servertest.CookieNamed(t, servertest.PostForm(t, env.Handler, "/login", correct, nil, nil), servertest.DeviceCookie)

	for range 5 {
		servertest.PostForm(t, env.Handler, "/login", wrong, nil, nil)
	}

	stranger := servertest.PostForm(t, env.Handler, "/login", correct, nil, nil)
	assert.Contains(t, stranger.Body.String(), "Слишком много неудачных попыток")

	owner := servertest.PostForm(t, env.Handler, "/login", correct, []*http.Cookie{device}, nil)
	servertest.AssertRedirect(t, owner, "/")
	assert.NotEmpty(t, servertest.CookieNamed(t, owner, servertest.SessionCookie).Value)
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
