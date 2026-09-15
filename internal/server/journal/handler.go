package journal

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const journalPath = "/journal"

type Handler struct {
	base    *web.Base
	auth    *auth.Service
	journal *journal.Service
}

func New(base *web.Base, authService *auth.Service, journalService *journal.Service) *Handler {
	return &Handler{base: base, auth: authService, journal: journalService}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET "+journalPath, h.teacher(h.index))
}

func (h *Handler) teacher(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleTeacher)(fn))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	h.base.Render(w, r, pages.Stub(view.StubPage{Shell: h.base.Shell(r, "Журнал", journalPath), Heading: "Журнал"}))
}
