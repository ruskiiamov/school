package journal

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	summaryPath      = journalPath + "/summary"
	adminSummaryPath = adminJournalPath + "/summary"
)

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := web.UserFromContext(ctx)
	pair := r.URL.Query().Get("pair")
	classID, subjectID := parsePair(pair)

	pairs, err := h.journal.Pairs(ctx, user.ID, h.school.CurrentYear(), h.school.Today())
	if err != nil {
		h.base.ServerError(w, r, "list teacher pairs", err)
		return
	}

	page := view.GridPage{
		Shell: h.base.Shell(r, "Сводка", summaryPath),
		Title: "Сводка",
		Path:  summaryPath,
	}

	for _, item := range pairs {
		value := pairValue(item.ClassID, item.SubjectID)
		page.Pairs = append(page.Pairs, view.PairOption{Value: value, Name: item.ClassName + " · " + item.SubjectName, Selected: value == pair})
	}

	if !hasPair(pairs, classID, subjectID) {
		classID, subjectID = 0, 0
	}

	hidden := map[string]string{}
	if classID != 0 {
		hidden["pair"] = pair
	}

	period := h.period(r)
	page.Period = web.PeriodForm(period, h.school.Today(), summaryPath, hidden)

	if classID != 0 {
		page.Selected = true

		grid, err := h.journal.TeacherGrid(ctx, user.ID, classID, subjectID, period.From, period.To)
		if err != nil {
			h.base.ServerError(w, r, "load teacher grid", err)
			return
		}

		page.Grid, err = h.gridView(r, grid, func(lessonID, studentID int64) string {
			return studentURL(lessonID, studentID, 0)
		})
		if err != nil {
			h.base.ServerError(w, r, "list students", err)
			return
		}
	}

	h.renderGrid(w, r, page)
}

func (h *Handler) adminSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, subjectID := queryID(r, "class"), queryID(r, "subject")

	page := view.GridPage{
		Shell: h.base.Shell(r, "Сводка", adminJournalPath),
		Title: "Сводка",
		Path:  adminSummaryPath,
	}

	if err := h.pairOptions(r, &page.Classes, &page.Subjects, classID, subjectID); err != nil {
		h.base.ServerError(w, r, "list classes and subjects", err)
		return
	}

	hidden := map[string]string{}
	if classID != 0 && subjectID != 0 {
		hidden["class"] = strconv.FormatInt(classID, 10)
		hidden["subject"] = strconv.FormatInt(subjectID, 10)
	}

	period := h.period(r)
	page.Period = web.PeriodForm(period, h.school.Today(), adminSummaryPath, hidden)

	if classID != 0 && subjectID != 0 {
		page.Selected = true

		grid, err := h.journal.PairGrid(ctx, classID, subjectID, period.From, period.To)
		if err != nil {
			h.base.ServerError(w, r, "load pair grid", err)
			return
		}

		page.Grid, err = h.gridView(r, grid, adminLessonURL)
		if err != nil {
			h.base.ServerError(w, r, "list students", err)
			return
		}
	}

	h.renderGrid(w, r, page)
}

func (h *Handler) period(r *http.Request) web.Period {
	start, end := h.school.YearBounds(h.school.CurrentYear())

	return web.ParsePeriod(r, h.school.Today(), start, end)
}

func (h *Handler) pairOptions(r *http.Request, classes, subjects *[]view.Option, classID, subjectID int64) error {
	ctx := r.Context()

	classList, err := h.school.Classes(ctx, h.school.CurrentYear(), false)
	if err != nil {
		return err
	}

	subjectList, err := h.school.Subjects(ctx, false)
	if err != nil {
		return err
	}

	*classes = []view.Option{{Name: classOption, Selected: classID == 0}}
	for _, class := range classList {
		*classes = append(*classes, view.Option{ID: class.ID, Name: class.Name, Selected: class.ID == classID})
	}

	*subjects = []view.Option{{Name: subjectOption, Selected: subjectID == 0}}
	for _, subject := range subjectList {
		*subjects = append(*subjects, view.Option{ID: subject.ID, Name: subject.Name, Selected: subject.ID == subjectID})
	}

	return nil
}

func (h *Handler) gridView(r *http.Request, grid journal.Grid, lessonURL func(lessonID, studentID int64) string) (*view.Grid, error) {
	students, err := h.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return nil, err
	}

	names := make(map[int64]auth.User, len(students))
	for _, student := range students {
		names[student.ID] = student
	}

	sort.Slice(grid.Rows, func(i, j int) bool {
		return names[grid.Rows[i].StudentID].FullName < names[grid.Rows[j].StudentID].FullName
	})

	result := &view.Grid{}

	for _, lesson := range grid.Lessons {
		result.Columns = append(result.Columns, view.GridColumn{Date: lesson.Date.Format("02.01"), Href: lessonURL(lesson.ID, 0)})
	}

	for _, row := range grid.Rows {
		user := names[row.StudentID]
		item := view.GridRow{
			FullName:  user.FullName,
			ShortName: view.ShortName(user.FullName),
			Active:    user.Active,
			InClass:   row.InClass,
			Absences:  strconv.Itoa(row.Absences),
			Average:   view.FormatAverage(row.Average, row.MarkCount),
		}

		for _, lesson := range grid.Lessons {
			item.Cells = append(item.Cells, view.GridCell{
				Text: cellText(row.Cells[lesson.ID]),
				Href: lessonURL(lesson.ID, row.StudentID),
			})
		}

		result.Rows = append(result.Rows, item)
	}

	return result, nil
}

func (h *Handler) renderGrid(w http.ResponseWriter, r *http.Request, page view.GridPage) {
	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.SummaryContent(page))
		return
	}

	h.base.Render(w, r, pages.Summary(page))
}

func cellText(cell journal.GridCell) string {
	values := make([]string, 0, len(cell.Marks))
	for _, value := range cell.Marks {
		values = append(values, strconv.Itoa(value))
	}

	text := strings.Join(values, ", ")

	switch {
	case cell.Absent && text != "":
		return text + " · Н"
	case cell.Absent:
		return "Н"
	default:
		return text
	}
}

func hasPair(pairs []journal.Pair, classID, subjectID int64) bool {
	for _, pair := range pairs {
		if pair.ClassID == classID && pair.SubjectID == subjectID {
			return true
		}
	}

	return false
}
