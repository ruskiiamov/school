package server

import (
	"bytes"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/logger"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/storage"
)

const (
	adminLogin    = "admin"
	adminPassword = "secret"
)

type testEnv struct {
	handler http.Handler
	db      *sql.DB
	auth    *auth.Service
	school  *school.Service
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	return newTestEnvWithLogger(t, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	return newTestEnv(t).handler
}

func newTestServerWithLog(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}
	env := newTestEnvWithLogger(t, slog.New(logger.NewContextHandler(slog.NewJSONHandler(buf, nil))))

	return env.handler, buf
}

func newTestEnvWithLogger(t *testing.T, log *slog.Logger) *testEnv {
	t.Helper()

	cfg := &config.Config{
		School:   config.School{Name: "Школа №1", YearStartMonth: time.August},
		Session:  config.Session{CookieName: "sid", TTL: time.Hour},
		Admin:    config.Admin{Login: adminLogin, Password: adminPassword, FullName: "Иванова Мария Петровна"},
		Location: time.UTC,
	}

	db, err := storage.Open(t.Context(), config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, storage.Migrate(t.Context(), db))

	authService := auth.NewService(storage.NewUserRepo(db), storage.NewSessionRepo(db), cfg.Session.TTL, log)
	require.NoError(t, authService.EnsureAdmin(t.Context(), cfg.Admin))

	schoolService := school.NewService(cfg.School.YearStartMonth, cfg.Location,
		storage.NewSubjectRepo(db), storage.NewWorkTypeRepo(db), storage.NewClassRepo(db),
		storage.NewClassStudentRepo(db), storage.NewParentChildRepo(db), storage.NewAssignmentRepo(db), log)

	return &testEnv{
		handler: New(cfg, authService, schoolService, log).Handler(),
		db:      db,
		auth:    authService,
		school:  schoolService,
	}
}

func (env *testEnv) createUser(t *testing.T, role auth.Role, login, fullName string) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(login+"-password"), bcrypt.MinCost)
	require.NoError(t, err)

	_, err = storage.NewUserRepo(env.db).Create(t.Context(), storage.User{
		Login:        login,
		PasswordHash: string(hash),
		FullName:     fullName,
		Role:         string(role),
		Active:       true,
	})
	require.NoError(t, err)
}

func (env *testEnv) loginAs(t *testing.T, login string) *http.Cookie {
	t.Helper()

	return loginWith(t, env.handler, login, login+"-password")
}

func login(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()

	return loginWith(t, handler, adminLogin, adminPassword)
}

func loginWith(t *testing.T, handler http.Handler, login, password string) *http.Cookie {
	t.Helper()

	recorder := postForm(t, handler, "/login",
		url.Values{"login": {login}, "password": {password}},
		nil, map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "/", recorder.Header().Get("HX-Redirect"))

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)

	return cookies[0]
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

func get(t *testing.T, handler http.Handler, path string, options ...any) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)

	for _, option := range options {
		switch option := option.(type) {
		case *http.Cookie:
			req.AddCookie(option)
		case map[string]string:
			for name, value := range option {
				req.Header.Set(name, value)
			}
		default:
			t.Fatalf("unsupported request option %T", option)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func assertRedirect(t *testing.T, recorder *httptest.ResponseRecorder, target string) {
	t.Helper()

	require.Equal(t, http.StatusSeeOther, recorder.Code, recorder.Body.String())
	require.Equal(t, target, recorder.Header().Get("Location"))
}
