package server

import (
	"log/slog"
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/view/static"
)

type Server struct {
	auth       *auth.Service
	schoolName string
	cookie     config.Session
	log        *slog.Logger
}

func New(cfg *config.Config, authService *auth.Service, log *slog.Logger) *Server {
	return &Server{
		auth:       authService,
		schoolName: cfg.School.Name,
		cookie:     cfg.Session,
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
