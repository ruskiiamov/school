package journal

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	journalPath     = "/journal"
	lessonsPath     = journalPath + "/lessons"
	homePath        = "/"
	lessonsPageSize = 10
)

type Handler struct {
	base    *web.Base
	auth    *auth.Service
	school  *school.Service
	journal *journal.Service
	files   config.Files
}

func New(base *web.Base, authService *auth.Service, schoolService *school.Service, journalService *journal.Service, files config.Files) *Handler {
	return &Handler{base: base, auth: authService, school: schoolService, journal: journalService, files: files}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET "+journalPath, h.teacher(h.index))
	mux.Handle("POST "+journalPath, h.teacher(h.open))
	mux.Handle("GET "+journalPath+"/class/{class}/subject/{subject}", h.teacher(h.pairShow))
	mux.Handle("GET "+lessonsPath+"/{id}", h.teacher(h.lessonShow))
	mux.Handle("POST "+lessonsPath+"/{id}/topic", h.teacher(h.lessonTopic))
	mux.Handle("POST "+lessonsPath+"/{id}/delete", h.teacher(h.lessonDelete))
	mux.Handle("POST "+lessonsPath+"/{id}/marks", h.teacher(h.markAdd))
	mux.Handle("POST "+lessonsPath+"/{id}/marks/{mid}", h.teacher(h.markUpdate))
	mux.Handle("POST "+lessonsPath+"/{id}/marks/{mid}/delete", h.teacher(h.markDelete))
	mux.Handle("POST "+lessonsPath+"/{id}/students/{sid}", h.teacher(h.recordSave))
	mux.Handle("POST "+lessonsPath+"/{id}/homework", h.teacher(h.homeworkSave))
	mux.Handle("POST "+lessonsPath+"/{id}/homework/files", h.teacher(h.homeworkUpload))
	mux.Handle("POST "+lessonsPath+"/{id}/homework/files/{fid}/delete", h.teacher(h.homeworkFileDelete))
	mux.Handle("GET "+summaryPath, h.teacher(h.summary))
	mux.Handle("GET "+adminJournalPath, h.admin(h.adminIndex))
	mux.Handle("GET "+adminSummaryPath, h.admin(h.adminSummary))
	mux.Handle("GET "+adminLessonsPath+"/{id}", h.admin(h.adminLesson))
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

func (h *Handler) Dashboard(r *http.Request, teacherID int64) (view.TeacherHome, error) {
	year, years, err := web.YearSelection(r, h.school)
	if err != nil {
		return view.TeacherHome{}, err
	}

	pairs, err := h.journal.YearPairs(r.Context(), teacherID, year)
	if err != nil {
		return view.TeacherHome{}, err
	}

	home := view.TeacherHome{Path: homePath, Years: years}

	for _, pair := range pairs {
		card := pairCard(pair)
		if pair.Kind == journal.PairAssigned {
			home.Assigned = append(home.Assigned, card)
		} else {
			home.Substitutions = append(home.Substitutions, card)
		}
	}

	return home, nil
}

func pairCard(pair journal.YearPair) view.PairCard {
	card := view.PairCard{
		Href:    pairPath(pair.ClassID, pair.SubjectID),
		Name:    pair.ClassName + " · " + pair.SubjectName,
		Lessons: "Уроков пока нет",
		OwnOnly: pair.Kind == journal.PairOwnLessons,
	}

	if pair.LessonCount > 0 {
		card.Lessons = view.Plural(pair.LessonCount, "урок", "урока", "уроков")
	}

	for _, period := range pair.Periods {
		card.Periods = append(card.Periods, view.FormatPeriod(period.Start, period.End))
	}

	return card
}

func pairPath(classID, subjectID int64) string {
	return journalPath + "/class/" + strconv.FormatInt(classID, 10) + "/subject/" + strconv.FormatInt(subjectID, 10)
}

func teacherPairOptions(pairs []journal.Pair, selected string) []view.PairOption {
	options := make([]view.PairOption, 0, len(pairs))

	for _, pair := range pairs {
		value := pairValue(pair.ClassID, pair.SubjectID)
		options = append(options, view.PairOption{
			Value:    value,
			Name:     pair.ClassName + " · " + pair.SubjectName,
			Selected: value == selected,
		})
	}

	return options
}

func lessonRows(lessons []journal.Lesson) []view.LessonRow {
	rows := make([]view.LessonRow, 0, len(lessons))

	for _, lesson := range lessons {
		rows = append(rows, view.LessonRow{
			Href:    lessonPath(lesson.ID, ""),
			Date:    view.FormatShortDate(lesson.Date),
			Class:   lesson.ClassName,
			Subject: lesson.SubjectName,
			Topic:   lesson.Topic,
		})
	}

	return rows
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

	number := requestedPage(r)

	lessons, total, err := h.journal.LessonsPage(ctx, user.ID, year, number, lessonsPageSize)
	if err != nil {
		h.base.ServerError(w, r, "list lessons", err)
		return
	}

	pageCount := max(1, (total+lessonsPageSize-1)/lessonsPageSize)
	if number > pageCount {
		web.Redirect(w, r, journalPageURL(pageCount))
		return
	}

	page := view.JournalPage{
		Shell:   h.base.Shell(r, "Журнал", journalPath),
		Date:    form.date,
		Errors:  form.errs,
		Pairs:   teacherPairOptions(pairs, form.pair),
		Lessons: lessonRows(lessons),
		Pager:   lessonsPager(number, pageCount),
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.JournalContent(page))
		return
	}

	h.base.Render(w, r, pages.Journal(page))
}

func requestedPage(r *http.Request) int {
	number, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || number < 1 {
		return 1
	}

	return number
}

func journalPageURL(number int) string {
	if number <= 1 {
		return journalPath
	}

	return journalPath + "?page=" + strconv.Itoa(number)
}

func lessonsPager(number, pageCount int) view.Pager {
	pager := view.Pager{Label: "Страница " + strconv.Itoa(number) + " из " + strconv.Itoa(pageCount), Pages: pageCount}

	if number > 1 {
		pager.NewerHref = journalPageURL(number - 1)
	}

	if number < pageCount {
		pager.OlderHref = journalPageURL(number + 1)
	}

	return pager
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
