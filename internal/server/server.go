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
	created    *credentialsStore
	log        *slog.Logger
}

func New(cfg *config.Config, authService *auth.Service, schoolService *school.Service, log *slog.Logger) *Server {
	return &Server{
		auth:       authService,
		school:     schoolService,
		schoolName: cfg.School.Name,
		cookie:     cfg.Session,
		location:   cfg.Location,
		created:    newCredentialsStore(),
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

	account := requireRole(auth.RoleTeacher, auth.RoleStudent, auth.RoleParent)
	mux.Handle("GET "+accountPasswordPath, s.requireAuth(account(http.HandlerFunc(s.passwordPage))))
	mux.Handle("POST "+accountPasswordPath, s.requireAuth(account(http.HandlerFunc(s.passwordSubmit))))

	mux.Handle("GET /journal", s.requireAuth(requireRole(auth.RoleTeacher)(http.HandlerFunc(s.journalStub))))
	mux.Handle("GET /diary", s.requireAuth(requireRole(auth.RoleStudent, auth.RoleParent)(http.HandlerFunc(s.diaryStub))))

	mux.Handle("GET /admin/classes", s.admin(s.classesList))
	mux.Handle("POST /admin/classes", s.admin(s.classCreate))
	mux.Handle("GET /admin/classes/{id}", s.admin(s.classShow))
	mux.Handle("POST /admin/classes/{id}", s.admin(s.classUpdate))
	mux.Handle("POST /admin/classes/{id}/deactivate", s.admin(s.classDeactivate))
	mux.Handle("POST /admin/classes/{id}/activate", s.admin(s.classActivate))
	mux.Handle("POST /admin/classes/{id}/students", s.admin(s.classStudentAdd))
	mux.Handle("POST /admin/classes/{id}/students/{sid}/remove", s.admin(s.classStudentRemove))
	mux.Handle("POST /admin/classes/{id}/assignments", s.admin(s.classAssign))
	mux.Handle("POST /admin/classes/{id}/assignments/{aid}/remove", s.admin(s.classAssignmentRemove))

	for _, section := range userSections {
		mux.Handle("GET "+section.path, s.admin(s.usersList(section)))
		mux.Handle("POST "+section.path, s.admin(s.userCreate(section)))
		mux.Handle("GET "+section.path+"/{id}/created", s.admin(s.userCreated(section)))
		mux.Handle("POST "+section.path+"/{id}", s.admin(s.userUpdate(section)))
		mux.Handle("POST "+section.path+"/{id}/deactivate", s.admin(s.userSetActive(section, false)))
		mux.Handle("POST "+section.path+"/{id}/activate", s.admin(s.userSetActive(section, true)))
	}

	mux.Handle("POST "+parentsSection.path+"/{id}/children", s.admin(s.parentChildAdd))
	mux.Handle("POST "+parentsSection.path+"/{id}/children/{sid}/remove", s.admin(s.parentChildRemove))

	mux.Handle("GET "+passwordResetPath, s.admin(s.passwordResetList))
	mux.Handle("POST "+passwordResetPath+"/{id}", s.admin(s.passwordReset))
	mux.Handle("GET "+passwordResetPath+"/{id}/created", s.admin(s.passwordResetCreated))

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
