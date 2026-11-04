package journal

import (
	"net/http"
	"sort"
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

type pairState struct {
	newDate     string
	newError    string
	rowID       int64
	rowDate     string
	dateError   string
	topic       string
	topicErr    string
	highlight   int64
	open        int64
	markValues  map[journal.MarkKey]string
	doneValues  map[journal.MarkKey]string
	markErrs    validation.Errors
	newTypeName string
	marksError  string
}

const (
	journalContentID = "journal-content"
	openLessonHeader = "X-Journal-Open"
)

func (h *Handler) pairShow(w http.ResponseWriter, r *http.Request) {
	classID, subjectID, ok := pairPathIDs(w, r)
	if !ok {
		return
	}

	h.renderPair(w, r, classID, subjectID, pairState{})
}

func (h *Handler) pairLessonCreate(w http.ResponseWriter, r *http.Request) {
	classID, subjectID, ok := pairPathIDs(w, r)
	if !ok {
		return
	}

	class, err := h.school.ClassByID(r.Context(), classID)
	if err != nil {
		h.base.HandleServiceError(w, r, "load class", err)
		return
	}

	user, _ := web.UserFromContext(r.Context())
	date := web.FormValue(r, "date")

	id, err := h.journal.CreateLesson(r.Context(), user.ID, classID, subjectID, date, h.yearBounds(class.Year))
	if errs, ok := web.FormErrors(err); ok {
		h.renderPair(w, r, classID, subjectID, pairState{newDate: date, newError: errs["date"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "create lesson", err)
		return
	}

	h.pairDone(w, r, classID, subjectID, id)
}

func (h *Handler) lessonMove(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	date := web.FormValue(r, "date")

	if date == "" {
		h.lessonClear(w, r, lesson)
		return
	}

	err := h.journal.MoveLesson(r.Context(), user.ID, lesson.ID, date, h.yearBounds(lesson.Year))
	if errs, ok := web.FormErrors(err); ok {
		h.renderPair(w, r, lesson.ClassID, lesson.SubjectID, pairState{rowID: lesson.ID, rowDate: date, dateError: errs["date"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "move lesson", err)
		return
	}

	h.pairDone(w, r, lesson.ClassID, lesson.SubjectID, lesson.ID)
}

func (h *Handler) lessonClear(w http.ResponseWriter, r *http.Request, lesson journal.Lesson) {
	user, _ := web.UserFromContext(r.Context())

	err := h.journal.DeleteLesson(r.Context(), user.ID, lesson.ID)
	if errs, ok := web.FormErrors(err); ok {
		h.renderPair(w, r, lesson.ClassID, lesson.SubjectID, pairState{rowID: lesson.ID, rowDate: lesson.Date.Format(validation.InputDateLayout), dateError: errs["lesson"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "delete lesson", err)
		return
	}

	if web.IsHTMX(r) {
		h.renderPair(w, r, lesson.ClassID, lesson.SubjectID, pairState{})
		return
	}

	web.Redirect(w, r, pairPath(lesson.ClassID, lesson.SubjectID))
}

func (h *Handler) lessonTopic(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	topic := web.FormValue(r, "topic")

	err := h.journal.UpdateTopic(r.Context(), user.ID, lesson.ID, topic)
	errs, invalid := web.FormErrors(err)
	if err != nil && !invalid {
		h.base.HandleServiceError(w, r, "update lesson topic", err)
		return
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.TopicError(topicTarget(lesson.ID)+"-error", errs["topic"]))
		return
	}

	if invalid {
		h.renderPair(w, r, lesson.ClassID, lesson.SubjectID, pairState{rowID: lesson.ID, rowDate: lesson.Date.Format(validation.InputDateLayout), topic: topic, topicErr: errs["topic"]})
		return
	}

	web.Redirect(w, r, pairPath(lesson.ClassID, lesson.SubjectID)+"#"+lessonAnchor(lesson.ID))
}

func (h *Handler) pairDone(w http.ResponseWriter, r *http.Request, classID, subjectID, lessonID int64) {
	if web.IsHTMX(r) {
		h.renderPair(w, r, classID, subjectID, pairState{highlight: lessonID})
		return
	}

	web.Redirect(w, r, pairPath(classID, subjectID)+"#"+lessonAnchor(lessonID))
}

func (h *Handler) yearBounds(year int) journal.YearBounds {
	start, end := h.school.YearBounds(year)

	return journal.YearBounds{Start: start, End: end}
}

func pairPathIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	classID, classOK := web.PathValue(r, "class")
	subjectID, subjectOK := web.PathValue(r, "subject")

	if !classOK || !subjectOK {
		http.NotFound(w, r)
		return 0, 0, false
	}

	return classID, subjectID, true
}

func lessonAnchor(lessonID int64) string {
	return "lesson-" + strconv.FormatInt(lessonID, 10)
}

func (h *Handler) renderPair(w http.ResponseWriter, r *http.Request, classID, subjectID int64, state pairState) {
	content, pair, err := h.pairContent(r, classID, subjectID, state)
	if err != nil {
		h.base.HandleServiceError(w, r, "load pair journal", err)
		return
	}

	if web.IsHTMX(r) && r.Header.Get("HX-Target") == journalContentID {
		h.base.Render(w, r, pages.PairJournalContent(content, false))
		return
	}

	title := pair.Pair.ClassName + " · " + pair.Pair.SubjectName

	page := view.PairJournalPage{
		Shell:     h.base.Shell(r, title, homePath),
		Title:     title + " · " + school.YearName(pair.Year),
		BackHref:  homePath,
		Penalties: h.school.PenaltySettings().Enabled,
		Content:   content,
	}

	if pair.Year != h.school.CurrentYear() {
		page.BackHref += "?year=" + strconv.Itoa(pair.Year)
	}

	h.base.Render(w, r, pages.PairJournal(page))
}

func (h *Handler) pairContent(r *http.Request, classID, subjectID int64, state pairState) (view.PairJournalContent, journal.PairJournal, error) {
	user, _ := web.UserFromContext(r.Context())

	pair, err := h.journal.PairJournal(r.Context(), user.ID, classID, subjectID)
	if err != nil {
		return view.PairJournalContent{}, journal.PairJournal{}, err
	}

	table, err := h.journalTable(r, pair, user.ID, func(lessonID, studentID int64) string {
		return studentMarksPath(lessonID, studentID)
	})
	if err != nil {
		return view.PairJournalContent{}, journal.PairJournal{}, err
	}

	content := view.PairJournalContent{
		Lessons: view.Plural(len(pair.Lessons), "урок", "урока", "уроков"),
		New:     newLessonForm(pair, state),
		Table:   table,
	}

	if pair.Pair.Kind == journal.PairSubstitution {
		content.Periods = substitutionPeriods(pair.Pair.Periods)
	}

	open := state.open
	if open == 0 && web.IsHTMX(r) {
		open, _ = strconv.ParseInt(r.Header.Get(openLessonHeader), 10, 64)
	}

	for i, lesson := range pair.Lessons {
		row := &content.Table.Rows[i]
		row.Highlight = lesson.ID == state.highlight
		row.Edit = rowEdit(lesson, state)
		row.CardHref = lessonPath(lesson.ID, "")
		row.MarksHref = lessonPath(lesson.ID, "/marks")

		if lesson.ID == state.rowID && state.topicErr != "" {
			row.Topic = state.topic
		}

		if lesson.ID != open {
			continue
		}

		marks, err := h.journal.LessonMarks(r.Context(), user.ID, lesson.ID)
		if err != nil {
			return view.PairJournalContent{}, journal.PairJournal{}, err
		}

		row.Expanded = true
		row.Marks = lessonMarksBlock(marks, pair.Students, content.Table.Students, state, h.school.PenaltySettings())
	}

	return content, pair, nil
}

func newLessonForm(pair journal.PairJournal, state pairState) *view.NewLessonForm {
	return &view.NewLessonForm{
		Action: pairPath(pair.Pair.ClassID, pair.Pair.SubjectID) + "/lessons",
		Date:   state.newDate,
		Error:  state.newError,
	}
}

func rowEdit(lesson journal.Lesson, state pairState) *view.JournalRowEdit {
	edit := &view.JournalRowEdit{
		DateAction:  lessonPath(lesson.ID, "/date"),
		DateValue:   lesson.Date.Format(validation.InputDateLayout),
		TopicAction: lessonPath(lesson.ID, "/topic"),
		TopicID:     topicTarget(lesson.ID),
	}

	if lesson.ID == state.rowID {
		edit.DateValue = state.rowDate
		edit.DateError = state.dateError
		edit.TopicError = state.topicErr
	}

	return edit
}

func topicTarget(lessonID int64) string {
	return "topic-" + strconv.FormatInt(lessonID, 10)
}

func substitutionPeriods(periods []journal.SubstitutionPeriod) string {
	parts := make([]string, 0, len(periods))
	for _, period := range periods {
		parts = append(parts, view.FormatPeriod(period.Start, period.End))
	}

	return "Ваши замены: " + strings.Join(parts, ", ")
}

func (h *Handler) journalTable(r *http.Request, pair journal.PairJournal, viewerID int64, cellHref func(lessonID, studentID int64) string) (view.JournalTable, error) {
	students, err := h.sortStudents(r, pair.Students)
	if err != nil {
		return view.JournalTable{}, err
	}

	teachers, err := h.teacherNames(r)
	if err != nil {
		return view.JournalTable{}, err
	}

	table := view.JournalTable{}
	settings := h.school.PenaltySettings()

	for _, student := range pair.Students {
		user := students[student.ID]
		table.Students = append(table.Students, view.JournalStudentHead{
			ID:        studentHeadID(student.ID),
			LastName:  user.Name.Last,
			FirstName: user.Name.First,
			FullName:  user.FullName,
			Active:    user.Active,
			InClass:   student.InClass,
			Average:   view.FormatAverage(student.Average, student.MarkCount),
			Absences:  strconv.Itoa(student.Absences) + "×Н",
		})
	}

	for _, lesson := range pair.Lessons {
		row := view.JournalRow{
			Anchor:   lessonAnchor(lesson.ID),
			Date:     view.FormatShortDate(lesson.Date),
			Topic:    lesson.Topic,
			Homework: pair.WithHomework[lesson.ID],
		}

		if lesson.TeacherID != viewerID {
			row.Author = view.ShortName(teachers[lesson.TeacherID])
		}

		for _, student := range pair.Students {
			cell := pair.Cells[journal.CellKey{LessonID: lesson.ID, StudentID: student.ID}]
			row.Cells = append(row.Cells, journalCell(cell, cellHref(lesson.ID, student.ID), settings))
		}

		table.Rows = append(table.Rows, row)
	}

	return table, nil
}

func (h *Handler) sortStudents(r *http.Request, list []journal.JournalStudent) (map[int64]auth.User, error) {
	students, err := h.studentsByID(r)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(list, func(i, j int) bool {
		return students[list[i].ID].FullName < students[list[j].ID].FullName
	})

	return students, nil
}

func (h *Handler) studentsByID(r *http.Request) (map[int64]auth.User, error) {
	users, err := h.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]auth.User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}

	return byID, nil
}

func journalCell(cell journal.JournalCell, href string, settings school.PenaltySettings) view.JournalCell {
	values := make([]string, 0, len(cell.Marks)+1)
	details := make([]string, 0, len(cell.Marks)+2)

	commented := cell.Comment != ""

	for _, mark := range cell.Marks {
		value := view.FormatMark(mark.Value)
		values = append(values, value)
		details = append(details, markDetail(mark.WorkTypeName, value, mark.Comment))
		commented = commented || mark.Comment != ""
	}

	if cell.Absent {
		values = append(values, "Н")
		details = append(details, "Отсутствовал")
	}

	penalized := settings.Enabled && len(cell.Penalties) > 0

	if penalized {
		for _, penalty := range cell.Penalties {
			details = append(details, penalty.WorkTypeName+": выполнено "+view.FormatPercent(penalty.DonePercent)+", "+
				view.FormatPoints(settings.Unit, penalty.Points))
		}
	}

	if cell.Comment != "" {
		details = append(details, cell.Comment)
	}

	return view.JournalCell{
		Href:    href,
		Text:    strings.Join(values, " "),
		Title:   strings.Join(details, " · "),
		Comment: commented,
		Penalty: penalized,
	}
}

func markDetail(workType, value, comment string) string {
	if comment == "" {
		return workType + ": " + value
	}

	return workType + ": " + value + " (" + comment + ")"
}
