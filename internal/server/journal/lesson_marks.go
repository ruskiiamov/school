package journal

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const markFieldPrefix = "m-"

func (h *Handler) lessonMarksShow(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	if !web.IsHTMX(r) {
		h.renderLessonMarksPage(w, r, lesson, pairState{open: lesson.ID}, nil)
		return
	}

	content, _, err := h.pairContent(r, lesson.ClassID, lesson.SubjectID, pairState{open: lesson.ID})
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson marks", err)
		return
	}

	row, found := findRow(content, lesson.ID)
	if !found || row.Marks == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("HX-Push-Url", "false")
	h.base.Render(w, r, pages.JournalRowGroup(row))
}

func (h *Handler) lessonMarksSave(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	values, keys := markValues(r)
	user, _ := web.UserFromContext(r.Context())
	state := pairState{open: lesson.ID}

	err := h.journal.SaveLessonMarks(r.Context(), user.ID, lesson.ID, values)
	errs, invalid := web.FormErrors(err)
	if err != nil && !invalid {
		h.base.HandleServiceError(w, r, "save lesson marks", err)
		return
	}

	if invalid {
		state.markValues = values
		state.markErrs = errs
	}

	if !web.IsHTMX(r) {
		if invalid {
			h.renderLessonMarksPage(w, r, lesson, state, nil)
			return
		}

		web.Redirect(w, r, pairPath(lesson.ClassID, lesson.SubjectID)+"#"+lessonAnchor(lesson.ID))

		return
	}

	h.renderMarkCells(w, r, lesson, state, keys, invalid)
}

func (h *Handler) renderMarkCells(w http.ResponseWriter, r *http.Request, lesson journal.Lesson, state pairState, keys []journal.MarkKey, invalid bool) {
	content, _, err := h.pairContent(r, lesson.ClassID, lesson.SubjectID, state)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson marks", err)
		return
	}

	row, found := findRow(content, lesson.ID)
	if !found || row.Marks == nil {
		http.NotFound(w, r)
		return
	}

	submitted := map[string]bool{}
	for _, key := range keys {
		submitted[markCellID(lesson.ID, key)] = true
	}

	var (
		cells   []view.LessonMarkCell
		message string
	)

	for _, marksRow := range row.Marks.Rows {
		for _, cell := range marksRow.Cells {
			if !submitted[cell.ID] {
				continue
			}

			cells = append(cells, cell)

			if cell.Error != "" && message == "" {
				message = cell.Label + ": " + cell.Error
			}
		}
	}

	if invalid {
		h.base.Render(w, r, pages.LessonMarkCells(row.Marks.Action, cells, nil, nil, row.Marks.ErrorID, message))
		return
	}

	students := map[string]bool{}
	for _, key := range keys {
		students[studentHeadID(key.StudentID)] = true
	}

	var heads []view.JournalStudentHead

	for _, head := range content.Table.Students {
		if students[head.ID] {
			heads = append(heads, head)
		}
	}

	h.base.Render(w, r, pages.LessonMarkCells(row.Marks.Action, cells, &row, heads, row.Marks.ErrorID, ""))
}

func (h *Handler) lessonWorkTypeCreate(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	name := web.FormValue(r, "name")

	_, err := h.journal.CreateLessonWorkType(r.Context(), user.ID, lesson.ID, name)
	errs, invalid := web.FormErrors(err)
	if err != nil && !invalid {
		h.base.HandleServiceError(w, r, "create lesson work type", err)
		return
	}

	state := pairState{open: lesson.ID}
	if invalid {
		state.newTypeName = name
		state.marksError = errs["name"]
	}

	if web.IsHTMX(r) {
		h.renderPair(w, r, lesson.ClassID, lesson.SubjectID, state)
		return
	}

	if invalid {
		h.renderLessonMarksPage(w, r, lesson, state, []string{errs["name"]})
		return
	}

	web.Redirect(w, r, lessonPath(lesson.ID, "/marks"))
}

func (h *Handler) renderLessonMarksPage(w http.ResponseWriter, r *http.Request, lesson journal.Lesson, state pairState, extra []string) {
	content, pair, err := h.pairContent(r, lesson.ClassID, lesson.SubjectID, state)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson marks", err)
		return
	}

	row, found := findRow(content, lesson.ID)
	if !found || row.Marks == nil {
		http.NotFound(w, r)
		return
	}

	title := pair.Pair.ClassName + " · " + pair.Pair.SubjectName + " · " + view.FormatShortDate(lesson.Date)

	page := view.LessonMarksPage{
		Shell:    h.base.Shell(r, title, homePath),
		Title:    title,
		BackHref: pairPath(lesson.ClassID, lesson.SubjectID) + "#" + lessonAnchor(lesson.ID),
		Block:    *row.Marks,
		Errors:   extra,
	}

	page.Block.Error = ""

	for i, student := range pair.Students {
		head := content.Table.Students[i]
		page.Students = append(page.Students, view.LessonMarksStudent{
			Name: strings.TrimSpace(head.LastName + " " + head.FirstName),
			Href: studentMarksPath(lesson.ID, student.ID),
		})
	}

	for _, marksRow := range page.Block.Rows {
		for _, cell := range marksRow.Cells {
			if cell.Error != "" {
				page.Errors = append(page.Errors, cell.Label+": "+cell.Error)
			}
		}
	}

	h.base.Render(w, r, pages.LessonMarksPage(page))
}

func lessonMarksBlock(marks journal.LessonMarks, students []journal.JournalStudent, heads []view.JournalStudentHead, state pairState) *view.LessonMarksBlock {
	lessonID := marks.Lesson.ID

	block := &view.LessonMarksBlock{
		Lesson:  lessonAnchor(lessonID),
		Columns: max(len(students), 1),
		Action:  lessonPath(lessonID, "/marks"),
		ErrorID: lessonAnchor(lessonID) + "-marks-error",
		Error:   state.marksError,
	}

	if marks.CanAddType {
		block.NewType = &view.NewWorkTypeForm{Action: lessonPath(lessonID, "/work-types"), Name: state.newTypeName}
	}

	for _, workType := range marks.Types {
		row := view.LessonMarksRow{Name: workType.Name, Active: workType.Active}

		for i, student := range students {
			key := journal.MarkKey{WorkTypeID: workType.ID, StudentID: student.ID}
			mark := marks.Marks[key]

			cell := view.LessonMarkCell{
				ID:       markCellID(lessonID, key),
				Field:    key.Field(),
				Label:    strings.TrimSpace(heads[i].LastName+" "+heads[i].FirstName) + " · " + workType.Name,
				Value:    view.FormatMark(mark.Value),
				Comment:  mark.Comment,
				Editable: marks.Editable(key),
			}

			if typed, ok := state.markValues[key]; ok {
				cell.Value = typed
				cell.Error = state.markErrs[key.Field()]
			}

			row.Cells = append(row.Cells, cell)
		}

		block.Rows = append(block.Rows, row)
	}

	return block
}

func markValues(r *http.Request) (map[journal.MarkKey]string, []journal.MarkKey) {
	values := map[journal.MarkKey]string{}
	var keys []journal.MarkKey

	for field := range r.PostForm {
		raw, found := strings.CutPrefix(field, markFieldPrefix)
		if !found {
			continue
		}

		typePart, studentPart, found := strings.Cut(raw, "-")
		if !found {
			continue
		}

		typeID, typeErr := strconv.ParseInt(typePart, 10, 64)
		studentID, studentErr := strconv.ParseInt(studentPart, 10, 64)

		if typeErr != nil || studentErr != nil {
			continue
		}

		key := journal.MarkKey{WorkTypeID: typeID, StudentID: studentID}
		values[key] = web.FormValue(r, field)
		keys = append(keys, key)
	}

	return values, keys
}

func findRow(content view.PairJournalContent, lessonID int64) (view.JournalRow, bool) {
	anchor := lessonAnchor(lessonID)

	for _, row := range content.Table.Rows {
		if row.Anchor == anchor {
			return row, true
		}
	}

	return view.JournalRow{}, false
}

func markCellID(lessonID int64, key journal.MarkKey) string {
	return lessonAnchor(lessonID) + "-" + key.Field()
}

func studentHeadID(studentID int64) string {
	return "student-" + strconv.FormatInt(studentID, 10)
}
