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
	adminDiaryPath           = "/admin/diary"

	classOption   = "Класс"
	subjectOption = "Предмет"
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
		if err := h.adminJournalContent(r, &page, classID, subjectID, adminOpenLesson(r)); err != nil {
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

func adminOpenLesson(r *http.Request) int64 {
	if web.IsHTMX(r) {
		open, _ := strconv.ParseInt(r.Header.Get(openLessonHeader), 10, 64)
		return open
	}

	return queryID(r, "open")
}

func (h *Handler) adminJournalContent(r *http.Request, page *view.AdminJournalsPage, classID, subjectID, open int64) error {
	pair, err := h.journal.AdminPairJournal(r.Context(), classID, subjectID)
	if err != nil {
		return err
	}

	table, err := h.journalTable(r, pair, pair.TeacherID, adminEntryPath)
	if err != nil {
		return err
	}

	for i, lesson := range pair.Lessons {
		row := &table.Rows[i]
		row.MarksHref = adminLessonPath(lesson.ID) + "/marks"

		if pair.WithHomework[lesson.ID] {
			row.CardHref = adminLessonPath(lesson.ID)
		}

		if lesson.ID != open {
			continue
		}

		marks, err := h.journal.AdminLessonMarks(r.Context(), lesson.ID)
		if err != nil {
			return err
		}

		lesson.Year = pair.Year
		row.Expanded = true
		row.CollapseHref = adminJournalsURL(lesson, 0)
		row.Marks = lessonMarksBlock(marks, pair.Students, table.Students, pairState{}, h.school.PenaltySettings())
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

func (h *Handler) adminJournalsMarks(w http.ResponseWriter, r *http.Request) {
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

	if !web.IsHTMX(r) {
		web.Redirect(w, r, adminJournalsURL(lesson, lesson.ID))
		return
	}

	var page view.AdminJournalsPage
	if err := h.adminJournalContent(r, &page, lesson.ClassID, lesson.SubjectID, lesson.ID); err != nil {
		h.base.HandleServiceError(w, r, "load lesson marks", err)
		return
	}

	row, found := findRow(*page.Content, lesson.ID)
	if !found {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("HX-Push-Url", "false")
	h.base.Render(w, r, pages.JournalRowGroup(row))
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
		BackHref: adminJournalsURL(lesson, 0),
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
		BackHref:  adminJournalsURL(lesson, 0),
		DiaryHref: adminDiaryPath + "/" + strconv.FormatInt(studentID, 10) + "?" + diary.Encode(),
		InClass:   entry.InClass,
		Absent:    entry.Absent,
		Comment:   entry.Comment,
	}

	settings := h.school.PenaltySettings()

	for _, workType := range entry.Types {
		mark := view.DiaryMark{Value: view.FormatMark(workType.Value), WorkType: workType.Name, Comment: workType.Comment}
		if settings.Enabled && workType.Penalty != nil {
			mark.Penalty = web.PenaltySummary(settings, *workType.Penalty)
		}

		if mark.Value != "" || mark.Comment != "" || mark.Penalty != "" {
			result.Marks = append(result.Marks, mark)
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
	return adminLessonPath(lessonID) + "/students/" + strconv.FormatInt(studentID, 10)
}

func adminLessonPath(lessonID int64) string {
	return adminJournalsLessonsPath + "/" + strconv.FormatInt(lessonID, 10)
}

func adminJournalsURL(lesson journal.Lesson, open int64) string {
	query := url.Values{
		"year":    {strconv.Itoa(lesson.Year)},
		"class":   {strconv.FormatInt(lesson.ClassID, 10)},
		"subject": {strconv.FormatInt(lesson.SubjectID, 10)},
	}

	if open != 0 {
		query.Set("open", strconv.FormatInt(open, 10))
	}

	return adminJournalsPath + "?" + query.Encode() + "#" + lessonAnchor(lesson.ID)
}

func (h *Handler) pairOptions(r *http.Request, classes, subjects *[]view.Option, year int, classID, subjectID int64) error {
	ctx := r.Context()

	classList, err := h.school.Classes(ctx, year, false)
	if err != nil {
		return err
	}

	*classes = []view.Option{{Name: classOption}}
	for _, class := range classList {
		*classes = append(*classes, view.Option{ID: class.ID, Name: view.ClassTitle(class.Name), Selected: class.ID == classID})
	}

	*subjects = []view.Option{{Name: subjectOption}}

	if selectedOption(*classes) {
		subjectList, err := h.journal.ClassSubjects(ctx, classID)
		if err != nil {
			return err
		}

		for _, subject := range subjectList {
			*subjects = append(*subjects, view.Option{ID: subject.ID, Name: subject.Name, Selected: subject.ID == subjectID})
		}
	}

	(*classes)[0].Selected = !selectedOption(*classes)
	(*subjects)[0].Selected = !selectedOption(*subjects)

	return nil
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
