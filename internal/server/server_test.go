package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/storage"
)

const (
	adminLogin    = "admin"
	adminPassword = "secret"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	cfg := &config.Config{
		School:  config.School{Name: "Школа №1"},
		Session: config.Session{CookieName: "sid", TTL: time.Hour},
		Admin:   config.Admin{Login: adminLogin, Password: adminPassword, FullName: "Иванова Мария Петровна"},
	}

	db, err := storage.Open(t.Context(), config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, storage.Migrate(db))

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authService := auth.NewService(storage.NewUserRepo(db), storage.NewSessionRepo(db), cfg.Session.TTL, log)
	require.NoError(t, authService.EnsureAdmin(t.Context(), cfg.Admin))

	return New(cfg, authService, log).Handler()
}

func postForm(t *testing.T, handler http.Handler, path string, form url.Values, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func login(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()

	recorder := postForm(t, handler, "/login",
		url.Values{"login": {adminLogin}, "password": {adminPassword}},
		nil, map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "/", recorder.Header().Get("HX-Redirect"))

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)

	return cookies[0]
}

func get(t *testing.T, handler http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func TestHomeRedirectsAnonymousUser(t *testing.T) {
	t.Parallel()

	recorder := get(t, newTestServer(t), "/")

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Location"))
}

func TestHomeRedirectsHtmxRequestWithHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")

	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("HX-Redirect"))
}

func TestLoginPageRenders(t *testing.T) {
	t.Parallel()

	recorder := get(t, newTestServer(t), "/login")
	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, body, `lang="ru"`)
	assert.Contains(t, body, `name="viewport"`)
	assert.Contains(t, body, "Электронный журнал")
	assert.Contains(t, body, "Школа №1")
	assert.Contains(t, body, `autocomplete="current-password"`)
}

func TestLoginSuccessSetsSecureCookie(t *testing.T) {
	t.Parallel()

	cookie := login(t, newTestServer(t))

	assert.Equal(t, "sid", cookie.Name)
	assert.NotEmpty(t, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, "/", cookie.Path)
	assert.Positive(t, cookie.MaxAge)
}

func TestLoginFailureReturnsFragmentWithoutCookie(t *testing.T) {
	t.Parallel()

	recorder := postForm(t, newTestServer(t), "/login",
		url.Values{"login": {adminLogin}, "password": {"wrong"}},
		nil, map[string]string{"HX-Request": "true"})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Result().Cookies())
	assert.Contains(t, recorder.Body.String(), "Неверный логин или пароль")
	assert.NotContains(t, recorder.Body.String(), "<html")
}

func TestLoginFailureWithoutHtmxRendersFullPage(t *testing.T) {
	t.Parallel()

	recorder := postForm(t, newTestServer(t), "/login",
		url.Values{"login": {adminLogin}, "password": {"wrong"}}, nil, nil)

	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "<html")
	assert.Contains(t, body, "Неверный логин или пароль")
	assert.Contains(t, body, `value="admin"`)
}

func TestLoginPageRedirectsAuthenticatedUser(t *testing.T) {
	t.Parallel()

	handler := newTestServer(t)
	recorder := get(t, handler, "/login", login(t, handler))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Location"))
}

func TestHomeRendersDashboardForAuthenticatedUser(t *testing.T) {
	t.Parallel()

	handler := newTestServer(t)
	recorder := get(t, handler, "/", login(t, handler))
	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "Здравствуйте, Иванова Мария Петровна")
	assert.Contains(t, body, "Администратор")
	assert.Contains(t, body, "Журнал оценок")
	assert.Contains(t, body, `hx-post="/logout"`)
}

func TestLogoutClearsCookieAndSession(t *testing.T) {
	t.Parallel()

	handler := newTestServer(t)
	cookie := login(t, handler)

	recorder := postForm(t, handler, "/logout", nil, []*http.Cookie{cookie},
		map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("HX-Redirect"))

	cleared := recorder.Result().Cookies()
	require.Len(t, cleared, 1)
	assert.Empty(t, cleared[0].Value)
	assert.Negative(t, cleared[0].MaxAge)

	after := get(t, handler, "/", cookie)
	assert.Equal(t, http.StatusSeeOther, after.Code)
	assert.Equal(t, "/login", after.Header().Get("Location"))
}

func TestCrossOriginPostRejected(t *testing.T) {
	t.Parallel()

	recorder := postForm(t, newTestServer(t), "/login",
		url.Values{"login": {adminLogin}, "password": {adminPassword}},
		nil, map[string]string{"Origin": "https://evil.example"})

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Empty(t, recorder.Result().Cookies())
}

func TestSameOriginPostAccepted(t *testing.T) {
	t.Parallel()

	recorder := postForm(t, newTestServer(t), "/login",
		url.Values{"login": {adminLogin}, "password": {adminPassword}},
		nil, map[string]string{"Origin": "http://example.com", "HX-Request": "true"})

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	t.Parallel()

	assert.Equal(t, http.StatusNotFound, get(t, newTestServer(t), "/does-not-exist").Code)
}

func TestSecurityHeadersAndStaticAssets(t *testing.T) {
	t.Parallel()

	handler := newTestServer(t)

	recorder := get(t, handler, "/healthz")
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Contains(t, recorder.Header().Get("Content-Security-Policy"), "script-src 'self'")
	assert.NotEmpty(t, recorder.Header().Get("X-Request-Id"))

	for _, path := range []string{
		"/static/css/app.css",
		"/static/js/htmx.min.js",
		"/static/js/app.js",
		"/static/favicon.svg",
	} {
		asset := get(t, handler, path)
		assert.Equal(t, http.StatusOK, asset.Code, path)
		assert.Equal(t, "no-cache", asset.Header().Get("Cache-Control"), path)
		assert.NotEmpty(t, asset.Body.String(), path)
	}
}

func newTestServerWithLog(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()

	cfg := &config.Config{
		School:  config.School{Name: "Школа №1"},
		Session: config.Session{CookieName: "sid", TTL: time.Hour},
		Admin:   config.Admin{Login: adminLogin, Password: adminPassword, FullName: "Иванова Мария Петровна"},
	}

	db, err := storage.Open(t.Context(), config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, storage.Migrate(db))

	buf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(buf, nil))
	authService := auth.NewService(storage.NewUserRepo(db), storage.NewSessionRepo(db), cfg.Session.TTL, log)
	require.NoError(t, authService.EnsureAdmin(t.Context(), cfg.Admin))

	return New(cfg, authService, log).Handler(), buf
}

func TestStaticAndHealthAreNotLogged(t *testing.T) {
	t.Parallel()

	handler, buf := newTestServerWithLog(t)

	for _, path := range []string{"/static/css/app.css", "/static/js/htmx.min.js", "/healthz"} {
		buf.Reset()

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		require.Equal(t, http.StatusOK, recorder.Code, path)
		assert.NotContains(t, buf.String(), `"msg":"request"`, path)
	}
}

func TestRequestLogHasReadableDuration(t *testing.T) {
	t.Parallel()

	handler, buf := newTestServerWithLog(t)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/login", nil))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.NotEmpty(t, lines)

	var entry struct {
		Msg      string `json:"msg"`
		Path     string `json:"path"`
		Duration string `json:"duration"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &entry))

	assert.Equal(t, "request", entry.Msg)
	assert.Equal(t, "/login", entry.Path)
	assert.Regexp(t, `^[0-9.]+(ns|µs|ms|s)$`, entry.Duration)
}

func TestEveryRequestLogCarriesRequestID(t *testing.T) {
	t.Parallel()

	handler, buf := newTestServerWithLog(t)

	form := url.Values{"login": {adminLogin}, "password": {"wrong"}}
	postForm(t, handler, "/login", form, nil, nil)

	var entries []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))

		if entry["msg"] == "request" || entry["msg"] == "invalid login attempt" {
			entries = append(entries, entry)
		}
	}

	require.Len(t, entries, 2)

	ids := make(map[string]bool)

	for _, entry := range entries {
		id, ok := entry["request_id"].(string)
		require.True(t, ok, entry["msg"])
		assert.NotEmpty(t, id)

		ids[id] = true
	}

	assert.Len(t, ids, 1)
}
