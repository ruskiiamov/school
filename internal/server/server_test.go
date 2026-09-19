package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/view/static"
)

func TestHomeRedirectsAnonymousUser(t *testing.T) {
	t.Parallel()

	recorder := servertest.Get(t, servertest.New(t).Handler, "/")

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Location"))
}

func TestHomeRedirectsHtmxRequestWithHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")

	recorder := httptest.NewRecorder()
	servertest.New(t).Handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("HX-Redirect"))
}

func TestHomeRendersDashboardForAuthenticatedUser(t *testing.T) {
	t.Parallel()

	handler := servertest.New(t).Handler
	recorder := servertest.Get(t, handler, "/", servertest.Login(t, handler))
	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "Здравствуйте, Мария Петровна")
	assert.Contains(t, body, "Иванова М. П.")
	assert.Contains(t, body, "Администратор")
	assert.Contains(t, body, `href="/admin/classes"`)
	assert.Contains(t, body, "Типы работ")
	assert.Contains(t, body, `<form method="post" action="/logout" hx-post="/logout">`)
}

func TestCrossOriginPostRejected(t *testing.T) {
	t.Parallel()

	recorder := servertest.PostForm(t, servertest.New(t).Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}},
		nil, map[string]string{"Origin": "https://evil.example"})

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Empty(t, recorder.Result().Cookies())
}

func TestPostWithoutOriginHeadersAccepted(t *testing.T) {
	t.Parallel()

	recorder := servertest.PostForm(t, servertest.New(t).Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}},
		nil, map[string]string{"HX-Request": "true"})

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestCrossSiteFetchRejected(t *testing.T) {
	t.Parallel()

	recorder := servertest.PostForm(t, servertest.New(t).Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}},
		nil, map[string]string{"Sec-Fetch-Site": "cross-site"})

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Empty(t, recorder.Result().Cookies())
}

func TestSameOriginPostAccepted(t *testing.T) {
	t.Parallel()

	recorder := servertest.PostForm(t, servertest.New(t).Handler, "/login",
		url.Values{"login": {servertest.AdminLogin}, "password": {servertest.AdminPassword}},
		nil, map[string]string{"Origin": "http://example.com", "HX-Request": "true"})

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	t.Parallel()

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, servertest.New(t).Handler, "/does-not-exist").Code)
}

func TestSecurityHeadersAndStaticAssets(t *testing.T) {
	t.Parallel()

	handler := servertest.New(t).Handler

	recorder := servertest.Get(t, handler, "/healthz")
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Contains(t, recorder.Header().Get("Content-Security-Policy"), "script-src 'self'")
	assert.NotEmpty(t, recorder.Header().Get("X-Request-Id"))

	for _, name := range []string{"css/app.css", "js/htmx.min.js", "js/app.js", "favicon.svg"} {
		asset := servertest.Get(t, handler, static.URL(name))
		assert.Equal(t, http.StatusOK, asset.Code, name)
		assert.Equal(t, "public, max-age=31536000, immutable", asset.Header().Get("Cache-Control"), name)
		assert.NotEmpty(t, asset.Body.String(), name)
	}
}

func TestStaleStaticVersionIsServedWithoutCaching(t *testing.T) {
	t.Parallel()

	handler := servertest.New(t).Handler

	asset := servertest.Get(t, handler, "/static/stale000/css/app.css")
	assert.Equal(t, http.StatusOK, asset.Code)
	assert.Equal(t, "no-cache", asset.Header().Get("Cache-Control"))

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, handler, static.URL("missing.txt")).Code)
}

func TestPagesLinkVersionedStaticAssets(t *testing.T) {
	t.Parallel()

	body := servertest.Get(t, servertest.New(t).Handler, "/login").Body.String()

	assert.Contains(t, body, `href="`+static.URL("css/app.css")+`"`)
	assert.Contains(t, body, `src="`+static.URL("js/htmx.min.js")+`"`)
	assert.NotContains(t, body, `"/static/css/app.css"`)
}

func TestStaticAndHealthAreNotLogged(t *testing.T) {
	t.Parallel()

	handler, buf := servertest.NewWithLog(t)

	for _, path := range []string{static.URL("css/app.css"), static.URL("js/htmx.min.js"), "/healthz"} {
		buf.Reset()

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		require.Equal(t, http.StatusOK, recorder.Code, path)
		assert.NotContains(t, buf.String(), `"msg":"request"`, path)
	}
}

func TestRequestLogHasReadableDuration(t *testing.T) {
	t.Parallel()

	handler, buf := servertest.NewWithLog(t)

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

	handler, buf := servertest.NewWithLog(t)

	form := url.Values{"login": {servertest.AdminLogin}, "password": {"wrong"}}
	servertest.PostForm(t, handler, "/login", form, nil, nil)

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
