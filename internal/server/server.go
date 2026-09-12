package server

import (
	"log/slog"
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/view/static"
)

const (
	staticPrefix = "/static/"
	healthPath   = "/healthz"
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
		log:        slog.New(contextHandler{Handler: log.Handler()}),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET "+staticPrefix, staticHandler())
	mux.HandleFunc("GET "+healthPath, s.health)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.Handle("POST /login", s.checkOrigin(http.HandlerFunc(s.loginSubmit)))
	mux.Handle("POST /logout", s.checkOrigin(http.HandlerFunc(s.logout)))
	mux.Handle("GET /{$}", s.requireAuth(http.HandlerFunc(s.home)))

	return chain(mux, requestID, s.logRequests, secureHeaders, s.recoverPanic)
}

func staticHandler() http.Handler {
	files := http.FileServerFS(static.FS)

	return http.StripPrefix(staticPrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	}))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("ok")); err != nil {
		s.log.ErrorContext(r.Context(), "write health response", slog.Any("error", err))
	}
}
