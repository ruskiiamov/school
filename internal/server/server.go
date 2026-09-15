package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/account"
	"github.com/ruskiiamov/school/internal/server/admin"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view/static"
)

type Server struct {
	base     *web.Base
	auth     *auth.Service
	school   *school.Service
	account  *account.Handler
	admin    *admin.Handler
	location *time.Location
	log      *slog.Logger
}

func New(cfg *config.Config, authService *auth.Service, schoolService *school.Service, log *slog.Logger) *Server {
	base := web.New(cfg, authService, log)

	return &Server{
		base:     base,
		auth:     authService,
		school:   schoolService,
		account:  account.New(base, authService, log),
		admin:    admin.New(base, authService, schoolService),
		location: cfg.Location,
		log:      log,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET "+static.Prefix+"{version}/{path...}", staticHandler())
	mux.HandleFunc("GET /healthz", s.health)
	mux.Handle("/", chain(s.pages(), s.logRequests, s.recoverPanic))

	return chain(mux, requestID, secureHeaders, s.crossOriginProtection)
}

func (s *Server) pages() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /{$}", s.base.RequireAuth(http.HandlerFunc(s.home)))
	mux.Handle("GET /journal", s.base.RequireAuth(web.RequireRole(auth.RoleTeacher)(http.HandlerFunc(s.journalStub))))
	mux.Handle("GET /diary", s.base.RequireAuth(web.RequireRole(auth.RoleStudent, auth.RoleParent)(http.HandlerFunc(s.diaryStub))))

	s.account.Routes(mux)
	s.admin.Routes(mux)

	return mux
}

func staticHandler() http.Handler {
	files := http.FileServerFS(static.FS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := r.PathValue("version")

		cacheControl := "no-cache"
		if version == static.Version() {
			cacheControl = "public, max-age=31536000, immutable"
		}

		w.Header().Set("Cache-Control", cacheControl)
		http.StripPrefix(static.Prefix+version, files).ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("ok")); err != nil {
		s.log.ErrorContext(r.Context(), "write health response", slog.Any("error", err))
	}
}
