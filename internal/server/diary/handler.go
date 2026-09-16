package diary

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const diaryPath = "/diary"

type Handler struct {
	base    *web.Base
	auth    *auth.Service
	school  *school.Service
	journal *journal.Service
}

func New(base *web.Base, authService *auth.Service, schoolService *school.Service, journalService *journal.Service) *Handler {
	return &Handler{base: base, auth: authService, school: schoolService, journal: journalService}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET "+diaryPath, h.owner(h.index))
}

func (h *Handler) owner(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleStudent, auth.RoleParent)(fn))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	h.base.Render(w, r, pages.Diary(view.DiaryPage{Shell: h.base.Shell(r, "Дневник", diaryPath)}))
}
