package journal

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	adminJournalsPath        = "/admin/journals"
	adminJournalsLessonsPath = adminJournalsPath + "/lessons"
	adminJournalsTarget      = "admin-journals"
)

func (h *Handler) adminJournals(w http.ResponseWriter, r *http.Request) {
	classID, subjectID := queryID(r, "class"), queryID(r, "subject")

	year, years, err := web.YearSelection(r, h.school)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	page := view.AdminJournalsPage{
		Shell: h.base.Shell(r, "Журналы", adminJournalsPath),
		Path:  adminJournalsPath,
		Years: years,
	}

	if err := h.pairOptions(r, &page.Classes, &page.Subjects, year, classID, subjectID); err != nil {
		h.base.ServerError(w, r, "list classes and subjects", err)
		return
	}

	if selectedOption(page.Classes) && selectedOption(page.Subjects) {
		if err := h.adminJournalContent(r, &page, classID, subjectID); err != nil {
			h.base.HandleServiceError(w, r, "load admin journal", err)
			return
		}
	}

	if web.IsHTMX(r) && r.Header.Get("HX-Target") == adminJournalsTarget {
		h.base.Render(w, r, pages.AdminJournalsContent(page))
		return
	}

	h.base.Render(w, r, pages.AdminJournals(page))
}

func (h *Handler) adminJournalContent(r *http.Request, page *view.AdminJournalsPage, classID, subjectID int64) error {
	pair, err := h.journal.AdminPairJournal(r.Context(), classID, subjectID)
	if err != nil {
		return err
	}

	table, err := h.journalTable(r, pair, pair.TeacherID, adminEntryPath)
	if err != nil {
		return err
	}

	for i, lesson := range pair.Lessons {
		if pair.WithHomework[lesson.ID] {
			table.Rows[i].CardHref = adminJournalsLessonsPath + "/" + strconv.FormatInt(lesson.ID, 10)
		}
	}

	page.Content = &view.PairJournalContent{
		Lessons: view.Plural(len(pair.Lessons), "урок", "урока", "уроков"),
		Table:   table,
	}

	page.Teacher = "Учитель не назначен"

	if pair.TeacherID != 0 {
		teachers, err := h.teacherNames(r)
		if err != nil {
			return err
		}

		page.Teacher = "Учитель: " + view.ShortName(teachers[pair.TeacherID])
	}

	return nil
}

func selectedOption(options []view.Option) bool {
	for _, option := range options {
		if option.Selected && option.ID != 0 {
			return true
		}
	}

	return false
}

func (h *Handler) adminJournalsCard(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	lesson, err := h.journal.LessonByID(r.Context(), id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson", err)
		return
	}

	homework, err := h.journal.Homework(r.Context(), lesson.ID)
	if err != nil {
		h.base.ServerError(w, r, "load homework", err)
		return
	}

	teachers, err := h.teacherNames(r)
	if err != nil {
		h.base.ServerError(w, r, "list teachers", err)
		return
	}

	card := view.AdminLessonCard{
		Title:    "Урок " + view.FormatShortDate(lesson.Date),
		BackHref: adminJournalsURL(lesson),
		Teacher:  "Провёл: " + view.ShortName(teachers[lesson.TeacherID]),
		Topic:    lesson.Topic,
	}

	if !homework.Empty() {
		card.Homework = diaryHomework(homework)
	}

	if web.IsHTMX(r) {
		w.Header().Set("HX-Push-Url", "false")
		h.base.Render(w, r, pages.AdminLessonWindow(card))

		return
	}

	h.base.Render(w, r, pages.AdminLessonCardPage(view.AdminLessonCardPage{Shell: h.base.Shell(r, card.Title, adminJournalsPath), Card: card}))
}

func (h *Handler) adminJournalsEntry(w http.ResponseWriter, r *http.Request) {
	lessonID, studentID, ok := entryPathIDs(w, r)
	if !ok {
		return
	}

	entry, err := h.journal.AdminMarksEntry(r.Context(), lessonID, studentID)
	if err != nil {
		h.base.HandleServiceError(w, r, "load marks", err)
		return
	}

	students, err := h.studentsByID(r)
	if err != nil {
		h.base.ServerError(w, r, "list students", err)
		return
	}

	student := students[studentID]
	lesson := entry.Lesson
	diary := url.Values{"date": {lesson.Date.Format(validation.DateLayout)}, "lesson": {strconv.FormatInt(lesson.ID, 10)}}

	result := view.AdminMarksEntry{
		Title:     strings.TrimSpace(student.Name.Last+" "+student.Name.First) + " · " + view.FormatShortDate(lesson.Date),
		BackHref:  adminJournalsURL(lesson),
		DiaryHref: adminDiaryPath + "/" + strconv.FormatInt(studentID, 10) + "?" + diary.Encode(),
		InClass:   entry.InClass,
		Absent:    entry.Absent,
		Comment:   entry.Comment,
	}

	for _, workType := range entry.Types {
		if workType.Value != 0 {
			result.Marks = append(result.Marks, view.DiaryMark{Value: strconv.Itoa(workType.Value), WorkType: workType.Name})
		}
	}

	if web.IsHTMX(r) {
		w.Header().Set("HX-Push-Url", "false")
		h.base.Render(w, r, pages.AdminMarksEntryWindow(result))

		return
	}

	h.base.Render(w, r, pages.AdminMarksEntryPage(view.AdminMarksEntryPage{Shell: h.base.Shell(r, result.Title, adminJournalsPath), Entry: result}))
}

func adminEntryPath(lessonID, studentID int64) string {
	return adminJournalsLessonsPath + "/" + strconv.FormatInt(lessonID, 10) + "/students/" + strconv.FormatInt(studentID, 10)
}

func adminJournalsURL(lesson journal.Lesson) string {
	query := url.Values{
		"year":    {strconv.Itoa(lesson.Year)},
		"class":   {strconv.FormatInt(lesson.ClassID, 10)},
		"subject": {strconv.FormatInt(lesson.SubjectID, 10)},
	}

	return adminJournalsPath + "?" + query.Encode() + "#" + lessonAnchor(lesson.ID)
}
