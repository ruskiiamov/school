package account

import (
	"log/slog"
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/web"
)

type Handler struct {
	base *web.Base
	auth *auth.Service
	log  *slog.Logger
}

func New(base *web.Base, authService *auth.Service, log *slog.Logger) *Handler {
	return &Handler{base: base, auth: authService, log: log}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.loginSubmit)
	mux.HandleFunc("POST /logout", h.logout)

	owner := web.RequireRole(auth.RoleTeacher, auth.RoleStudent, auth.RoleParent)
	mux.Handle("GET "+passwordPath, h.base.RequireAuth(owner(http.HandlerFunc(h.passwordPage))))
	mux.Handle("POST "+passwordPath, h.base.RequireAuth(owner(http.HandlerFunc(h.passwordSubmit))))
}
