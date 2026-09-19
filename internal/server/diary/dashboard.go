package diary

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
)

const dashboardDays = 7

type Dashboard struct {
	Children []view.DiaryChild
	Student  *view.StudentDashboard
}

func (h *Handler) Dashboard(r *http.Request, user auth.User) (Dashboard, error) {
	if user.Role == auth.RoleStudent {
		board, err := h.studentDashboard(r, user.ID, nil)
		if err != nil {
			return Dashboard{}, err
		}

		return Dashboard{Student: &board}, nil
	}

	children, err := h.children(r, user.ID)
	if err != nil || len(children) == 0 {
		return Dashboard{}, err
	}

	requested := queryID(r, "child")
	selected := children[0]
	found := requested == 0

	for _, child := range children {
		if child.ID == requested {
			selected, found = child, true
		}
	}

	if !found {
		return Dashboard{}, school.ErrNotFound
	}

	board, err := h.studentDashboard(r, selected.ID, childHidden(selected.ID))
	if err != nil {
		return Dashboard{}, err
	}

	result := Dashboard{Student: &board}

	for _, child := range children {
		result.Children = append(result.Children, view.DiaryChild{
			Name:   child.FullName,
			Href:   "/?child=" + strconv.FormatInt(child.ID, 10),
			Active: child.ID == selected.ID,
		})
	}

	return result, nil
}

func (h *Handler) studentDashboard(r *http.Request, studentID int64, hidden map[string]string) (view.StudentDashboard, error) {
	ctx := r.Context()
	today := h.school.Today()
	diary := diaryView{path: diaryPath, hidden: hidden}

	lessons, err := h.journal.DayLessons(ctx, studentID, today)
	if err != nil {
		return view.StudentDashboard{}, err
	}

	teacherNames, err := h.teacherNames(r)
	if err != nil {
		return view.StudentDashboard{}, err
	}

	summary, err := h.journal.StudentSummary(ctx, studentID, today.AddDate(0, 0, 1-dashboardDays), today)
	if err != nil {
		return view.StudentDashboard{}, err
	}

	board := view.StudentDashboard{
		DiaryHref: diaryURL(diary, today, 0),
		MarksHref: marksURL(hidden),
		Subjects:  subjectRows(summary, diary),
	}

	for _, lesson := range lessons {
		board.Lessons = append(board.Lessons, view.DiaryLessonItem{
			ID:       lesson.ID,
			Href:     diaryURL(diary, today, lesson.ID),
			Subject:  lesson.SubjectName,
			Teacher:  view.ShortName(teacherNames[lesson.TeacherID]),
			Summary:  diarySummary(lesson),
			Homework: !lesson.Homework.Empty(),
		})
	}

	return board, nil
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

func marksURL(hidden map[string]string) string {
	if len(hidden) == 0 {
		return summaryPath
	}

	query := url.Values{}
	for name, value := range hidden {
		query.Set(name, value)
	}

	return summaryPath + "?" + query.Encode()
}
