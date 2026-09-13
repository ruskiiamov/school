package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view/static"
)

type Server struct {
	auth       *auth.Service
	school     *school.Service
	schoolName string
	cookie     config.Session
	location   *time.Location
	log        *slog.Logger
}

func New(cfg *config.Config, authService *auth.Service, schoolService *school.Service, log *slog.Logger) *Server {
	return &Server{
		auth:       authService,
		school:     schoolService,
		schoolName: cfg.School.Name,
		cookie:     cfg.Session,
		location:   cfg.Location,
		log:        log,
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

	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.loginSubmit)
	mux.HandleFunc("POST /logout", s.logout)
	mux.Handle("GET /{$}", s.requireAuth(http.HandlerFunc(s.home)))

	mux.Handle("GET /journal", s.requireAuth(requireRole(auth.RoleTeacher)(http.HandlerFunc(s.journalStub))))
	mux.Handle("GET /diary", s.requireAuth(requireRole(auth.RoleStudent, auth.RoleParent)(http.HandlerFunc(s.diaryStub))))

	mux.Handle("GET /admin/classes", s.admin(s.classesList))
	mux.Handle("POST /admin/classes", s.admin(s.classCreate))
	mux.Handle("GET /admin/classes/{id}", s.admin(s.classShow))
	mux.Handle("POST /admin/classes/{id}", s.admin(s.classUpdate))
	mux.Handle("POST /admin/classes/{id}/deactivate", s.admin(s.classDeactivate))
	mux.Handle("POST /admin/classes/{id}/activate", s.admin(s.classActivate))

	mux.Handle("GET /admin/subjects", s.admin(s.subjectsList))
	mux.Handle("POST /admin/subjects", s.admin(s.subjectCreate))
	mux.Handle("POST /admin/subjects/{id}", s.admin(s.subjectUpdate))
	mux.Handle("POST /admin/subjects/{id}/deactivate", s.admin(s.subjectDeactivate))
	mux.Handle("POST /admin/subjects/{id}/activate", s.admin(s.subjectActivate))

	mux.Handle("GET /admin/work-types", s.admin(s.workTypesList))
	mux.Handle("POST /admin/work-types", s.admin(s.workTypeCreate))
	mux.Handle("POST /admin/work-types/{id}", s.admin(s.workTypeUpdate))
	mux.Handle("POST /admin/work-types/{id}/deactivate", s.admin(s.workTypeDeactivate))
	mux.Handle("POST /admin/work-types/{id}/activate", s.admin(s.workTypeActivate))
	mux.Handle("POST /admin/work-types/{id}/up", s.admin(s.workTypeUp))
	mux.Handle("POST /admin/work-types/{id}/down", s.admin(s.workTypeDown))

	return mux
}

func (s *Server) admin(h http.HandlerFunc) http.Handler {
	return s.requireAuth(requireRole(auth.RoleAdmin)(h))
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
