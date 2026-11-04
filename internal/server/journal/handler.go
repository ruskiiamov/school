package journal

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
)

const (
	journalPath = "/journal"
	lessonsPath = journalPath + "/lessons"
	homePath    = "/"
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
	mux.Handle("GET "+journalPath+"/class/{class}/subject/{subject}", h.teacher(h.pairShow))
	mux.Handle("POST "+journalPath+"/class/{class}/subject/{subject}/lessons", h.teacher(h.pairLessonCreate))
	mux.Handle("GET "+lessonsPath+"/{id}", h.teacher(h.lessonCard))
	mux.Handle("POST "+lessonsPath+"/{id}", h.teacher(h.lessonCardSave))
	mux.Handle("POST "+lessonsPath+"/{id}/date", h.teacher(h.lessonMove))
	mux.Handle("POST "+lessonsPath+"/{id}/topic", h.teacher(h.lessonTopic))
	mux.Handle("GET "+lessonsPath+"/{id}/marks", h.teacher(h.lessonMarksShow))
	mux.Handle("POST "+lessonsPath+"/{id}/marks", h.teacher(h.lessonMarksSave))
	mux.Handle("POST "+lessonsPath+"/{id}/work-types", h.teacher(h.lessonWorkTypeCreate))
	mux.Handle("GET "+lessonsPath+"/{id}/students/{sid}/marks", h.teacher(h.marksEntryShow))
	mux.Handle("POST "+lessonsPath+"/{id}/students/{sid}/marks", h.teacher(h.marksEntrySave))
	mux.Handle("GET "+workTypesPath, h.teacher(h.workTypesIndex))
	mux.Handle("GET "+journalPath+"/subjects/{subject}/work-types", h.teacher(h.workTypesList))
	mux.Handle("POST "+journalPath+"/subjects/{subject}/work-types", h.teacher(h.workTypeCreate))
	mux.Handle("POST "+journalPath+"/subjects/{subject}/work-types/{id}", h.teacher(h.workTypeUpdate))
	mux.Handle("POST "+journalPath+"/subjects/{subject}/work-types/{id}/deactivate", h.teacher(h.workTypeDeactivate))
	mux.Handle("POST "+journalPath+"/subjects/{subject}/work-types/{id}/activate", h.teacher(h.workTypeActivate))
	mux.Handle("POST "+journalPath+"/subjects/{subject}/work-types/{id}/up", h.teacher(h.workTypeUp))
	mux.Handle("POST "+journalPath+"/subjects/{subject}/work-types/{id}/down", h.teacher(h.workTypeDown))
	mux.Handle("GET "+adminJournalsPath, h.admin(h.adminJournals))
	mux.Handle("GET "+adminJournalsLessonsPath+"/{id}", h.admin(h.adminJournalsCard))
	mux.Handle("GET "+adminJournalsLessonsPath+"/{id}/students/{sid}", h.admin(h.adminJournalsEntry))
}

func (h *Handler) teacher(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleTeacher)(fn))
}

func (h *Handler) admin(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleAdmin)(fn))
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

func lessonPath(id int64, suffix string) string {
	return lessonsPath + "/" + strconv.FormatInt(id, 10) + suffix
}

func (h *Handler) pathLesson(w http.ResponseWriter, r *http.Request) (journal.Lesson, bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return journal.Lesson{}, false
	}

	user, _ := web.UserFromContext(r.Context())

	lesson, err := h.journal.LessonForTeacher(r.Context(), user.ID, id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson", err)
		return journal.Lesson{}, false
	}

	return lesson, true
}

func queryID(r *http.Request, name string) int64 {
	id, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}

	return id
}

func (h *Handler) teacherNames(r *http.Request) (map[int64]string, error) {
	teachers, err := h.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleTeacher, IncludeInactive: true})
	if err != nil {
		return nil, err
	}

	names := make(map[int64]string, len(teachers))
	for _, teacher := range teachers {
		names[teacher.ID] = teacher.FullName
	}

	return names, nil
}
