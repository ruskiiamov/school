package diary

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

type penaltiesView struct {
	title    string
	active   string
	student  int64
	diary    diaryView
	backHref string
	children []view.DiaryChild
}

func (h *Handler) penalties(w http.ResponseWriter, r *http.Request) {
	settings := h.school.PenaltySettings()
	if !settings.Enabled {
		http.NotFound(w, r)
		return
	}

	user, _ := web.UserFromContext(r.Context())
	child := queryID(r, "child")
	pv := penaltiesView{title: settings.Name, active: view.DiaryPenaltiesPath, diary: diaryView{path: diaryPath}}

	if user.Role != auth.RoleParent {
		if child != 0 && child != user.ID {
			http.NotFound(w, r)
			return
		}

		pv.student = user.ID
		h.renderPenalties(w, r, pv)

		return
	}

	selected, children, ok := h.selectChild(w, r, user.ID, child)
	if !ok {
		return
	}

	if len(children) == 0 {
		h.base.Render(w, r, pages.DiaryPenalties(view.DiaryPenaltiesPage{
			Shell:      h.base.Shell(r, pv.title, pv.active),
			Title:      pv.title,
			NoChildren: true,
		}))

		return
	}

	pv.student = selected.ID
	pv.diary.hidden = childHidden(selected.ID)

	for _, candidate := range children {
		pv.children = append(pv.children, view.DiaryChild{
			Name:   candidate.FullName,
			Href:   view.DiaryPenaltiesPath + "?child=" + strconv.FormatInt(candidate.ID, 10),
			Active: candidate.ID == selected.ID,
		})
	}

	h.renderPenalties(w, r, pv)
}

func (h *Handler) adminPenalties(w http.ResponseWriter, r *http.Request) {
	settings := h.school.PenaltySettings()
	if !settings.Enabled {
		http.NotFound(w, r)
		return
	}

	filter, err := h.adminFilter(r)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	student, title, ok := h.adminStudent(w, r, filter.year)
	if !ok {
		return
	}

	path := adminDiaryPath + "/" + strconv.FormatInt(student.ID, 10)

	h.renderPenalties(w, r, penaltiesView{
		title:    settings.Name + " · " + title,
		active:   adminDiaryPath,
		student:  student.ID,
		diary:    diaryView{path: path, hidden: filter.hidden()},
		backHref: path + filter.suffix(),
	})
}

func (h *Handler) renderPenalties(w http.ResponseWriter, r *http.Request, pv penaltiesView) {
	history, err := h.journal.PenaltyHistory(r.Context(), pv.student)
	if err != nil {
		h.base.ServerError(w, r, "load penalty history", err)
		return
	}

	recorders, err := h.recorderNames(r)
	if err != nil {
		h.base.ServerError(w, r, "list staff", err)
		return
	}

	h.base.Render(w, r, pages.DiaryPenalties(view.DiaryPenaltiesPage{
		Shell:    h.base.Shell(r, pv.title, pv.active),
		Title:    pv.title,
		BackHref: pv.backHref,
		Children: pv.children,
		History: web.PenaltyHistory(h.school.PenaltySettings(), history, web.HistoryLinks{
			Lesson: func(penalty journal.HistoryPenalty) string {
				return diaryURL(pv.diary, penalty.Date, penalty.LessonID)
			},
			Recorder: recorders,
		}),
	}))
}

func (h *Handler) recorderNames(r *http.Request) (map[int64]string, error) {
	names, err := h.teacherNames(r)
	if err != nil {
		return nil, err
	}

	admins, err := h.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleAdmin, IncludeInactive: true})
	if err != nil {
		return nil, err
	}

	for _, admin := range admins {
		names[admin.ID] = admin.FullName
	}

	return names, nil
}

func (h *Handler) penaltyCard(r *http.Request, studentID int64, hidden map[string]string) (*view.PenaltyCard, error) {
	settings := h.school.PenaltySettings()
	if !settings.Enabled {
		return nil, nil
	}

	debt, err := h.journal.StudentDebt(r.Context(), studentID)
	if err != nil || debt == 0 {
		return nil, err
	}

	href := view.DiaryPenaltiesPath
	if child := hidden["child"]; child != "" {
		href += "?child=" + child
	}

	return &view.PenaltyCard{Title: settings.Name, Debt: view.FormatDebt(settings.Unit, debt), Href: href}, nil
}

func diaryMarks(lesson journal.DiaryLesson, summary func(journal.Penalty) string) []view.DiaryMark {
	penalties := map[int64]journal.LessonPenalty{}
	if summary != nil {
		for _, penalty := range lesson.Penalties {
			penalties[penalty.WorkTypeID] = penalty
		}
	}

	marks := make([]view.DiaryMark, 0, len(lesson.Marks)+len(penalties))

	for _, mark := range lesson.Marks {
		item := view.DiaryMark{Value: view.FormatMark(mark.Value), WorkType: mark.WorkTypeName, Comment: mark.Comment}

		if penalty, found := penalties[mark.WorkTypeID]; found {
			item.Penalty = summary(penalty.Penalty)
			delete(penalties, mark.WorkTypeID)
		}

		marks = append(marks, item)
	}

	for _, penalty := range lesson.Penalties {
		if _, open := penalties[penalty.WorkTypeID]; open {
			marks = append(marks, view.DiaryMark{WorkType: penalty.WorkTypeName, Penalty: summary(penalty.Penalty)})
		}
	}

	return marks
}
