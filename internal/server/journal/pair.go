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
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (h *Handler) pairShow(w http.ResponseWriter, r *http.Request) {
	classID, classOK := web.PathValue(r, "class")
	subjectID, subjectOK := web.PathValue(r, "subject")

	if !classOK || !subjectOK {
		http.NotFound(w, r)
		return
	}

	user, _ := web.UserFromContext(r.Context())

	pair, err := h.journal.PairJournal(r.Context(), user.ID, classID, subjectID)
	if err != nil {
		h.base.HandleServiceError(w, r, "load pair journal", err)
		return
	}

	table, err := h.journalTable(r, pair, user.ID, func(lessonID, studentID int64) string {
		return studentURL(lessonID, studentID, 0)
	})
	if err != nil {
		h.base.ServerError(w, r, "build pair journal", err)
		return
	}

	title := pair.Pair.ClassName + " · " + pair.Pair.SubjectName

	page := view.PairJournalPage{
		Shell:    h.base.Shell(r, title, homePath),
		Title:    title + " · " + school.YearName(pair.Year),
		BackHref: homePath,
		Lessons:  view.Plural(len(pair.Lessons), "урок", "урока", "уроков"),
		Table:    table,
	}

	if pair.Year != h.school.CurrentYear() {
		page.BackHref += "?year=" + strconv.Itoa(pair.Year)
	}

	if pair.Pair.Kind == journal.PairSubstitution {
		page.Periods = substitutionPeriods(pair.Pair.Periods)
	}

	h.base.Render(w, r, pages.PairJournal(page))
}

func substitutionPeriods(periods []journal.SubstitutionPeriod) string {
	parts := make([]string, 0, len(periods))
	for _, period := range periods {
		parts = append(parts, view.FormatPeriod(period.Start, period.End))
	}

	return "Ваши замены: " + strings.Join(parts, ", ")
}

func (h *Handler) journalTable(r *http.Request, pair journal.PairJournal, viewerID int64, cellHref func(lessonID, studentID int64) string) (view.JournalTable, error) {
	students, err := h.studentsByID(r)
	if err != nil {
		return view.JournalTable{}, err
	}

	teachers, err := h.teacherNames(r)
	if err != nil {
		return view.JournalTable{}, err
	}

	sort.SliceStable(pair.Students, func(i, j int) bool {
		return students[pair.Students[i].ID].FullName < students[pair.Students[j].ID].FullName
	})

	table := view.JournalTable{}

	for _, student := range pair.Students {
		user := students[student.ID]
		table.Students = append(table.Students, view.JournalStudentHead{
			LastName:  user.Name.Last,
			FirstName: user.Name.First,
			FullName:  user.FullName,
			Active:    user.Active,
			InClass:   student.InClass,
			Average:   view.FormatAverage(student.Average, student.MarkCount),
			Absences:  student.Absences,
		})
	}

	for _, lesson := range pair.Lessons {
		row := view.JournalRow{
			Anchor:    "lesson-" + strconv.FormatInt(lesson.ID, 10),
			Date:      view.FormatShortDate(lesson.Date),
			ShortDate: lesson.Date.Format("02.01"),
			Topic:     lesson.Topic,
			Homework:  pair.WithHomework[lesson.ID],
		}

		if lesson.TeacherID != viewerID {
			row.Author = view.ShortName(teachers[lesson.TeacherID])
		}

		for _, student := range pair.Students {
			cell := pair.Cells[journal.CellKey{LessonID: lesson.ID, StudentID: student.ID}]
			row.Cells = append(row.Cells, journalCell(cell, cellHref(lesson.ID, student.ID)))
		}

		table.Rows = append(table.Rows, row)
	}

	return table, nil
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

func journalCell(cell journal.JournalCell, href string) view.JournalCell {
	values := make([]string, 0, len(cell.Marks)+1)
	details := make([]string, 0, len(cell.Marks)+2)

	for _, mark := range cell.Marks {
		values = append(values, strconv.Itoa(mark.Value))
		details = append(details, mark.WorkTypeName+": "+strconv.Itoa(mark.Value))
	}

	if cell.Absent {
		values = append(values, "Н")
		details = append(details, "Отсутствовал")
	}

	if cell.Comment != "" {
		details = append(details, cell.Comment)
	}

	return view.JournalCell{
		Href:    href,
		Text:    strings.Join(values, " "),
		Title:   strings.Join(details, " · "),
		Comment: cell.Comment != "",
	}
}
