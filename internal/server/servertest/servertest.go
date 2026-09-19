package servertest

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
	"github.com/ruskiiamov/school/internal/backup"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/files"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/logger"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server"
	"github.com/ruskiiamov/school/internal/storage"
)

const (
	AdminLogin    = "admin"
	AdminPassword = "secret"
)

type Env struct {
	Handler http.Handler
	DB      *sql.DB
	Auth    *auth.Service
	School  *school.Service
	Journal *journal.Service
	Files   *files.Store
}

func New(t *testing.T) *Env {
	t.Helper()

	return NewWithLogger(t, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func NewWithLog(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}
	env := NewWithLogger(t, slog.New(logger.NewContextHandler(slog.NewJSONHandler(buf, nil))))

	return env.Handler, buf
}

func NewWithLogger(t *testing.T, log *slog.Logger) *Env {
	t.Helper()

	cfg := &config.Config{
		School:   config.School{Name: "Школа №1", YearStartMonth: time.August},
		Session:  config.Session{CookieName: "sid", TTL: time.Hour},
		Files:    config.Files{Dir: t.TempDir(), MaxFileSizeMB: 1, MaxPerLesson: 3, TransferTimeout: time.Minute},
		Admin:    config.Admin{Login: AdminLogin, Password: AdminPassword, LastName: "Иванова", FirstName: "Мария", MiddleName: "Петровна"},
		Location: time.UTC,
	}

	store, err := files.NewStore(cfg.Files.Dir)
	require.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(t.Context(), config.DB{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, storage.Migrate(t.Context(), db))

	authService := auth.NewService(storage.NewUserRepo(db), storage.NewSessionRepo(db), cfg.Session.TTL, log)
	require.NoError(t, authService.EnsureAdmin(t.Context(), cfg.Admin))

	schoolService := school.NewService(cfg.School.YearStartMonth, cfg.Location,
		storage.NewSubjectRepo(db), storage.NewWorkTypeRepo(db), storage.NewClassRepo(db),
		storage.NewClassStudentRepo(db), storage.NewParentChildRepo(db), storage.NewAssignmentRepo(db),
		storage.NewSubstitutionRepo(db), log)

	journalService := journal.NewService(storage.NewLessonRepo(db), storage.NewMarkRepo(db), storage.NewLessonStudentRepo(db), storage.NewAssignmentRepo(db),
		storage.NewSubstitutionRepo(db), storage.NewClassRepo(db), storage.NewClassStudentRepo(db),
		storage.NewSubjectRepo(db), storage.NewWorkTypeRepo(db), storage.NewHomeworkRepo(db), storage.NewHomeworkFileRepo(db),
		store, journal.FileLimits{MaxFileSize: cfg.Files.MaxFileSize(), MaxPerLesson: cfg.Files.MaxPerLesson}, log)

	return &Env{
		Handler: server.New(cfg, authService, schoolService, journalService, backup.New(dbPath, store), log).Handler(),
		DB:      db,
		Auth:    authService,
		School:  schoolService,
		Journal: journalService,
		Files:   store,
	}
}

func (env *Env) CreateUser(t *testing.T, role auth.Role, login, fullName string) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(login+"-password"), bcrypt.MinCost)
	require.NoError(t, err)

	name := Name(fullName)

	_, err = storage.NewUserRepo(env.DB).Create(t.Context(), storage.User{
		Login:        login,
		PasswordHash: string(hash),
		LastName:     name.Last,
		FirstName:    name.First,
		MiddleName:   name.Middle,
		Role:         string(role),
		Active:       true,
	})
	require.NoError(t, err)
}

func Name(fullName string) auth.Name {
	parts := strings.Fields(fullName)
	name := auth.Name{}

	if len(parts) > 0 {
		name.Last = parts[0]
	}

	if len(parts) > 1 {
		name.First = parts[1]
	}

	if len(parts) > 2 {
		name.Middle = strings.Join(parts[2:], " ")
	}

	return name
}

func NameForm(fullName string) url.Values {
	name := Name(fullName)

	return url.Values{"last_name": {name.Last}, "first_name": {name.First}, "middle_name": {name.Middle}}
}

func (env *Env) LoginAs(t *testing.T, login string) *http.Cookie {
	t.Helper()

	return LoginWith(t, env.Handler, login, login+"-password")
}

func Login(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()

	return LoginWith(t, handler, AdminLogin, AdminPassword)
}

func LoginWith(t *testing.T, handler http.Handler, login, password string) *http.Cookie {
	t.Helper()

	recorder := PostForm(t, handler, "/login",
		url.Values{"login": {login}, "password": {password}},
		nil, map[string]string{"HX-Request": "true"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "/", recorder.Header().Get("HX-Redirect"))

	cookies := Cookies(t, recorder)
	require.Len(t, cookies, 1)

	return cookies[0]
}

func PostForm(t *testing.T, handler http.Handler, path string, form url.Values, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
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

func Get(t *testing.T, handler http.Handler, path string, options ...any) *httptest.ResponseRecorder {
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

func AssertRedirect(t *testing.T, recorder *httptest.ResponseRecorder, target string) {
	t.Helper()

	require.Equal(t, http.StatusSeeOther, recorder.Code, recorder.Body.String())
	require.Equal(t, target, recorder.Header().Get("Location"))
}

func Cookies(t *testing.T, recorder *httptest.ResponseRecorder) []*http.Cookie {
	t.Helper()

	response := recorder.Result()
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })

	return response.Cookies()
}
