package files

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
)

const filesPath = "/files"

type Handler struct {
	base    *web.Base
	school  *school.Service
	journal *journal.Service
	timeout time.Duration
	log     *slog.Logger
}

func New(base *web.Base, schoolService *school.Service, journalService *journal.Service, timeout time.Duration, log *slog.Logger) *Handler {
	return &Handler{base: base, school: schoolService, journal: journalService, timeout: timeout, log: log}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET "+filesPath+"/{id}", h.base.RequireAuth(http.HandlerFunc(h.download)))
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	file, err := h.journal.HomeworkFile(ctx, id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load homework file", err)
		return
	}

	user, _ := web.UserFromContext(ctx)

	allowed, err := h.allowed(r, user, file.LessonID)
	if err != nil {
		h.base.ServerError(w, r, "check file access", err)
		return
	}

	if !allowed {
		http.NotFound(w, r)
		return
	}

	content, err := h.journal.OpenHomeworkFile(ctx, id)
	if err != nil {
		h.base.HandleServiceError(w, r, "open homework file", err)
		return
	}
	defer func() { _ = content.Close() }()

	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(h.timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.base.ServerError(w, r, "extend write deadline", err)
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}))
	http.ServeContent(w, r, "", time.Time{}, content)
}

func (h *Handler) allowed(r *http.Request, user auth.User, lessonID int64) (bool, error) {
	ctx := r.Context()

	switch user.Role {
	case auth.RoleAdmin:
		return true, nil
	case auth.RoleTeacher:
		_, err := h.journal.LessonForTeacher(ctx, user.ID, lessonID)
		if errors.Is(err, journal.ErrForbidden) || errors.Is(err, journal.ErrNotFound) {
			return false, nil
		}

		return err == nil, err
	case auth.RoleStudent:
		return h.journal.LessonVisibleToStudent(ctx, user.ID, lessonID)
	case auth.RoleParent:
		links, err := h.school.Children(ctx)
		if err != nil {
			return false, err
		}

		for _, child := range links[user.ID] {
			visible, err := h.journal.LessonVisibleToStudent(ctx, child, lessonID)
			if err != nil || visible {
				return visible, err
			}
		}

		return false, nil
	default:
		return false, nil
	}
}
