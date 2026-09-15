package journal

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	journalPath = "/journal"
	lessonsPath = journalPath + "/lessons"
)

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
	mux.Handle("GET "+journalPath, h.teacher(h.index))
	mux.Handle("POST "+journalPath, h.teacher(h.open))
	mux.Handle("GET "+lessonsPath+"/{id}", h.teacher(h.lessonShow))
	mux.Handle("POST "+lessonsPath+"/{id}/topic", h.teacher(h.lessonTopic))
	mux.Handle("POST "+lessonsPath+"/{id}/delete", h.teacher(h.lessonDelete))
}

func (h *Handler) teacher(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleTeacher)(fn))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	h.renderJournal(w, r, journalForm{date: h.school.Today().Format(validation.DateLayout)})
}

type journalForm struct {
	pair string
	date string
	errs validation.Errors
}

func (h *Handler) open(w http.ResponseWriter, r *http.Request) {
	user, _ := web.UserFromContext(r.Context())
	form := journalForm{pair: web.FormValue(r, "pair"), date: web.FormValue(r, "date")}
	classID, subjectID := parsePair(form.pair)

	input := journal.OpenLessonInput{ClassID: classID, SubjectID: subjectID, Date: form.date}

	id, err := h.journal.OpenLesson(r.Context(), user.ID, h.school.CurrentYear(), h.school.Today(), input)
	if errs, ok := web.FormErrors(err); ok {
		form.errs = errs
		h.renderJournal(w, r, form)

		return
	}
	if err != nil {
		h.base.ServerError(w, r, "open lesson", err)
		return
	}

	web.Redirect(w, r, lessonPath(id, ""))
}

func (h *Handler) renderJournal(w http.ResponseWriter, r *http.Request, form journalForm) {
	ctx := r.Context()
	user, _ := web.UserFromContext(ctx)
	year := h.school.CurrentYear()

	pairs, err := h.journal.Pairs(ctx, user.ID, year, h.school.Today())
	if err != nil {
		h.base.ServerError(w, r, "list teacher pairs", err)
		return
	}

	lessons, err := h.journal.RecentLessons(ctx, user.ID, year)
	if err != nil {
		h.base.ServerError(w, r, "list recent lessons", err)
		return
	}

	page := view.JournalPage{
		Shell:  h.base.Shell(r, "Журнал", journalPath),
		Date:   form.date,
		Errors: form.errs,
	}

	for _, pair := range pairs {
		value := pairValue(pair.ClassID, pair.SubjectID)
		page.Pairs = append(page.Pairs, view.PairOption{
			Value:    value,
			Name:     pair.ClassName + " · " + pair.SubjectName,
			Selected: value == form.pair,
		})
	}

	for _, lesson := range lessons {
		page.Lessons = append(page.Lessons, view.LessonRow{
			Href:    lessonPath(lesson.ID, ""),
			Date:    view.FormatShortDate(lesson.Date),
			Class:   lesson.ClassName,
			Subject: lesson.SubjectName,
			Topic:   lesson.Topic,
		})
	}

	h.base.Render(w, r, pages.Journal(page))
}

func pairValue(classID, subjectID int64) string {
	return strconv.FormatInt(classID, 10) + "-" + strconv.FormatInt(subjectID, 10)
}

func parsePair(value string) (int64, int64) {
	classText, subjectText, ok := strings.Cut(value, "-")
	if !ok {
		return 0, 0
	}

	classID, err := strconv.ParseInt(classText, 10, 64)
	if err != nil {
		return 0, 0
	}

	subjectID, err := strconv.ParseInt(subjectText, 10, 64)
	if err != nil {
		return 0, 0
	}

	return classID, subjectID
}

func lessonPath(id int64, suffix string) string {
	return lessonsPath + "/" + strconv.FormatInt(id, 10) + suffix
}
