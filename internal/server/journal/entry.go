package journal

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

type entryForm struct {
	entered bool
	input   journal.MarksEntryInput
	errs    validation.Errors
}

func (h *Handler) marksEntryShow(w http.ResponseWriter, r *http.Request) {
	lessonID, studentID, ok := entryPathIDs(w, r)
	if !ok {
		return
	}

	h.renderEntry(w, r, lessonID, studentID, entryForm{})
}

func (h *Handler) marksEntrySave(w http.ResponseWriter, r *http.Request) {
	lessonID, studentID, ok := entryPathIDs(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	form := entryForm{entered: true, input: journal.MarksEntryInput{
		Values:  map[int64]string{},
		Absent:  r.PostFormValue("absent") == "1",
		Comment: web.FormValue(r, "comment"),
	}}

	for key := range r.PostForm {
		if id, err := strconv.ParseInt(strings.TrimPrefix(key, "type-"), 10, 64); err == nil && strings.HasPrefix(key, "type-") {
			form.input.Values[id] = web.FormValue(r, key)
		}
	}

	user, _ := web.UserFromContext(r.Context())

	err := h.journal.SaveMarksEntry(r.Context(), user.ID, lessonID, studentID, form.input)
	if errs, ok := web.FormErrors(err); ok {
		form.errs = errs
		h.renderEntry(w, r, lessonID, studentID, form)

		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "save marks", err)
		return
	}

	h.entryDone(w, r, lessonID)
}

func (h *Handler) entryDone(w http.ResponseWriter, r *http.Request, lessonID int64) {
	user, _ := web.UserFromContext(r.Context())

	lesson, err := h.journal.LessonForTeacher(r.Context(), user.ID, lessonID)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson", err)
		return
	}

	h.cardDone(w, r, lesson)
}

func (h *Handler) renderEntry(w http.ResponseWriter, r *http.Request, lessonID, studentID int64, form entryForm) {
	entry, err := h.entryView(r, lessonID, studentID, form)
	if err != nil {
		h.base.HandleServiceError(w, r, "load marks", err)
		return
	}

	if web.IsHTMX(r) {
		w.Header().Set("HX-Push-Url", "false")
		h.base.Render(w, r, pages.MarksEntryWindow(entry))

		return
	}

	h.base.Render(w, r, pages.MarksEntryPage(view.MarksEntryPage{Shell: h.base.Shell(r, entry.Title, homePath), Entry: entry}))
}

func (h *Handler) entryView(r *http.Request, lessonID, studentID int64, form entryForm) (view.MarksEntry, error) {
	user, _ := web.UserFromContext(r.Context())

	entry, err := h.journal.MarksEntry(r.Context(), user.ID, lessonID, studentID)
	if err != nil {
		return view.MarksEntry{}, err
	}

	lesson := entry.Lesson

	students, err := h.studentsByID(r)
	if err != nil {
		return view.MarksEntry{}, err
	}

	student := students[studentID]

	result := view.MarksEntry{
		Title:    strings.TrimSpace(student.Name.Last+" "+student.Name.First) + " · " + view.FormatShortDate(lesson.Date),
		Action:   studentMarksPath(lessonID, studentID),
		BackHref: pairPath(lesson.ClassID, lesson.SubjectID) + "#" + lessonAnchor(lessonID),
		InClass:  entry.InClass,
		Absent:   entry.Absent,
		Comment:  entry.Comment,
	}

	for i, workType := range entry.Types {
		row := view.MarksEntryRow{
			Name:      workType.Name,
			Field:     journal.EntryTypeKey(workType.ID),
			Autofocus: i == 0,
		}

		if workType.Value != 0 {
			row.Value = strconv.Itoa(workType.Value)
		}

		if form.entered {
			row.Value = form.input.Values[workType.ID]
			row.Error = form.errs[row.Field]
		}

		result.Rows = append(result.Rows, row)
	}

	if form.entered {
		result.Absent = form.input.Absent
		result.Comment = form.input.Comment
		result.CommentError = form.errs["comment"]
	}

	return result, nil
}

func entryPathIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	lessonID, lessonOK := web.PathID(r)
	studentID, studentOK := web.PathValue(r, "sid")

	if !lessonOK || !studentOK {
		http.NotFound(w, r)
		return 0, 0, false
	}

	return lessonID, studentID, true
}
