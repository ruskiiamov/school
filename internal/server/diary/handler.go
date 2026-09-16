package diary

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
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

type diaryView struct {
	title    string
	path     string
	student  int64
	hidden   map[string]string
	backHref string
	children []view.DiaryChild
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	user, _ := web.UserFromContext(r.Context())

	child := queryID(r, "child")
	if child != 0 && child != user.ID {
		http.NotFound(w, r)
		return
	}

	h.renderDiary(w, r, diaryView{title: "Дневник", path: diaryPath, student: user.ID})
}

func (h *Handler) renderDiary(w http.ResponseWriter, r *http.Request, dv diaryView) {
	ctx := r.Context()
	date := h.requestedDate(r)

	lessons, err := h.journal.DayLessons(ctx, dv.student, date)
	if err != nil {
		h.base.ServerError(w, r, "load diary lessons", err)
		return
	}

	teachers, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleTeacher, IncludeInactive: true})
	if err != nil {
		h.base.ServerError(w, r, "list teachers", err)
		return
	}

	teacherNames := make(map[int64]string, len(teachers))
	for _, teacher := range teachers {
		teacherNames[teacher.ID] = teacher.FullName
	}

	dateValue := date.Format(validation.DateLayout)

	page := view.DiaryPage{
		Shell:      h.base.Shell(r, dv.title, diaryPath),
		Title:      dv.title,
		BackHref:   dv.backHref,
		Children:   dv.children,
		Date:       dateValue,
		DateLabel:  view.FormatDate(date),
		DateAction: dv.path,
		Hidden:     dv.hidden,
		PrevHref:   diaryURL(dv, date.AddDate(0, 0, -1), 0),
		TodayHref:  diaryURL(dv, h.school.Today(), 0),
		NextHref:   diaryURL(dv, date.AddDate(0, 0, 1), 0),
	}

	requested := queryID(r, "lesson")
	selected := -1

	for i, lesson := range lessons {
		if lesson.ID == requested || (requested == 0 && i == 0) {
			selected = i
		}
	}

	if requested != 0 && selected < 0 {
		http.NotFound(w, r)
		return
	}

	for i, lesson := range lessons {
		page.Lessons = append(page.Lessons, view.DiaryLessonItem{
			ID:       lesson.ID,
			Href:     diaryURL(dv, date, lesson.ID),
			Subject:  lesson.SubjectName,
			Teacher:  view.ShortName(teacherNames[lesson.TeacherID]),
			Summary:  diarySummary(lesson),
			Selected: i == selected,
		})
	}

	if selected >= 0 {
		lesson := lessons[selected]
		panel := view.DiaryLessonPanel{
			Title:   lesson.SubjectName + " · " + view.FormatShortDate(lesson.Date),
			Teacher: teacherNames[lesson.TeacherID],
			Topic:   lesson.Topic,
			Absent:  lesson.Absent,
			Comment: lesson.Comment,
		}

		for _, mark := range lesson.Marks {
			panel.Marks = append(panel.Marks, view.DiaryMark{Value: strconv.Itoa(mark.Value), WorkType: mark.WorkTypeName, Label: mark.Label})
		}

		page.Selected = &panel
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.DiaryContent(page))
		return
	}

	h.base.Render(w, r, pages.Diary(page))
}

func (h *Handler) requestedDate(r *http.Request) time.Time {
	if date, ok := validation.ParseDate(r.URL.Query().Get("date")); ok {
		return date
	}

	return h.school.Today()
}

func diarySummary(lesson journal.DiaryLesson) string {
	summary := ""

	for i, mark := range lesson.Marks {
		if i > 0 {
			summary += ", "
		}

		summary += strconv.Itoa(mark.Value)
	}

	switch {
	case lesson.Absent && summary != "":
		return summary + " · Н"
	case lesson.Absent:
		return "Н"
	default:
		return summary
	}
}

func diaryURL(dv diaryView, date time.Time, lessonID int64) string {
	query := url.Values{"date": {date.Format(validation.DateLayout)}}

	for name, value := range dv.hidden {
		query.Set(name, value)
	}

	if lessonID != 0 {
		query.Set("lesson", strconv.FormatInt(lessonID, 10))
	}

	return dv.path + "?" + query.Encode()
}

func queryID(r *http.Request, name string) int64 {
	id, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}

	return id
}
