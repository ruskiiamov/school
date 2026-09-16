package journal

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	adminJournalPath = "/admin/journal"
	adminLessonsPath = adminJournalPath + "/lessons"
	adminDiaryPath   = "/admin/diary"

	classOption   = "Класс"
	subjectOption = "Предмет"
)

func (h *Handler) admin(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleAdmin)(fn))
}

func (h *Handler) adminIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, subjectID := queryID(r, "class"), queryID(r, "subject")

	classes, err := h.school.Classes(ctx, h.school.CurrentYear(), false)
	if err != nil {
		h.base.ServerError(w, r, "list classes", err)
		return
	}

	subjects, err := h.school.Subjects(ctx, false)
	if err != nil {
		h.base.ServerError(w, r, "list subjects", err)
		return
	}

	page := view.AdminJournalPage{
		Shell:    h.base.Shell(r, "Журналы", adminJournalPath),
		Path:     adminJournalPath,
		Classes:  []view.Option{{Name: classOption, Selected: classID == 0}},
		Subjects: []view.Option{{Name: subjectOption, Selected: subjectID == 0}},
	}

	for _, class := range classes {
		page.Classes = append(page.Classes, view.Option{ID: class.ID, Name: class.Name, Selected: class.ID == classID})
	}

	for _, subject := range subjects {
		page.Subjects = append(page.Subjects, view.Option{ID: subject.ID, Name: subject.Name, Selected: subject.ID == subjectID})
	}

	if classID != 0 && subjectID != 0 {
		page.Selected = true

		lessons, withHomework, err := h.journal.LessonsByPair(ctx, classID, subjectID)
		if err != nil {
			h.base.ServerError(w, r, "list pair lessons", err)
			return
		}

		teachers, err := h.teacherNames(r)
		if err != nil {
			h.base.ServerError(w, r, "list teachers", err)
			return
		}

		for _, lesson := range lessons {
			page.Lessons = append(page.Lessons, view.AdminLessonRow{
				Href:     adminLessonURL(lesson.ID, 0),
				Date:     view.FormatShortDate(lesson.Date),
				Teacher:  view.ShortName(teachers[lesson.TeacherID]),
				Topic:    lesson.Topic,
				Homework: withHomework[lesson.ID],
			})
		}
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.AdminJournalContent(page))
		return
	}

	h.base.Render(w, r, pages.AdminJournal(page))
}

func (h *Handler) adminLesson(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	lesson, err := h.journal.LessonByID(ctx, id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson", err)
		return
	}

	block, err := h.adminLessonBlock(r, lesson)
	if err != nil {
		h.base.ServerError(w, r, "load lesson students", err)
		return
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.AdminLessonBlock(block))
		return
	}

	teachers, err := h.teacherNames(r)
	if err != nil {
		h.base.ServerError(w, r, "list teachers", err)
		return
	}

	homework, err := h.journal.Homework(ctx, lesson.ID)
	if err != nil {
		h.base.ServerError(w, r, "load homework", err)
		return
	}

	title := lesson.ClassName + " · " + lesson.SubjectName + " · " + view.FormatShortDate(lesson.Date)
	back := url.Values{"class": {strconv.FormatInt(lesson.ClassID, 10)}, "subject": {strconv.FormatInt(lesson.SubjectID, 10)}}

	page := view.AdminLessonPage{
		Shell:    h.base.Shell(r, title, adminJournalPath),
		Title:    title,
		BackHref: adminJournalPath + "?" + back.Encode(),
		Teacher:  teachers[lesson.TeacherID],
		Topic:    lesson.Topic,
		Block:    block,
	}

	if !homework.Empty() {
		page.Homework = diaryHomework(homework)
	}

	h.base.Render(w, r, pages.AdminLesson(page))
}

func (h *Handler) adminLessonBlock(r *http.Request, lesson journal.Lesson) (view.AdminLessonBlock, error) {
	ctx := r.Context()

	entries, err := h.journal.LessonStudents(ctx, lesson)
	if err != nil {
		return view.AdminLessonBlock{}, err
	}

	students, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return view.AdminLessonBlock{}, err
	}

	names := make(map[int64]auth.User, len(students))
	for _, student := range students {
		names[student.ID] = student
	}

	sort.Slice(entries, func(i, j int) bool {
		return names[entries[i].ID].FullName < names[entries[j].ID].FullName
	})

	if len(entries) == 0 {
		return view.AdminLessonBlock{}, nil
	}

	selected := entries[0]
	requested := queryID(r, "student")

	for _, entry := range entries {
		if entry.ID == requested {
			selected = entry
		}
	}

	block := view.AdminLessonBlock{Students: make([]view.LessonStudentItem, 0, len(entries))}

	for _, entry := range entries {
		user := names[entry.ID]
		block.Students = append(block.Students, view.LessonStudentItem{
			ID:        entry.ID,
			Href:      adminLessonURL(lesson.ID, entry.ID),
			FullName:  user.FullName,
			ShortName: view.ShortName(user.FullName),
			Summary:   markSummary(entry),
			Active:    user.Active,
			InClass:   entry.InClass,
			Selected:  entry.ID == selected.ID,
		})
	}

	user := names[selected.ID]
	diary := url.Values{"date": {lesson.Date.Format(validation.DateLayout)}, "lesson": {strconv.FormatInt(lesson.ID, 10)}}
	panel := view.AdminStudentPanel{
		FullName:  user.FullName,
		Active:    user.Active,
		InClass:   selected.InClass,
		DiaryHref: adminDiaryPath + "/" + strconv.FormatInt(selected.ID, 10) + "?" + diary.Encode(),
		Absent:    selected.Absent,
		Comment:   selected.Comment,
	}

	for _, mark := range selected.Marks {
		panel.Marks = append(panel.Marks, view.DiaryMark{Value: strconv.Itoa(mark.Value), WorkType: mark.WorkTypeName, Label: mark.Label})
	}

	block.Selected = &panel

	return block, nil
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

func diaryHomework(homework journal.Homework) *view.DiaryHomework {
	result := &view.DiaryHomework{Text: homework.Text}

	if !homework.Due.IsZero() {
		result.Due = view.FormatShortDate(homework.Due)
	}

	for _, file := range homework.Files {
		result.Files = append(result.Files, fileLink(file))
	}

	return result
}

func adminLessonURL(lessonID, studentID int64) string {
	path := adminLessonsPath + "/" + strconv.FormatInt(lessonID, 10)
	if studentID == 0 {
		return path
	}

	return path + "?student=" + strconv.FormatInt(studentID, 10)
}
