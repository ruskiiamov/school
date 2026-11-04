package journal

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	entryValuePrefix   = "type-"
	entryCommentPrefix = "comment-"
	entryDonePrefix    = "done-"
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
		Values:   formFields(r, entryValuePrefix),
		Comments: formFields(r, entryCommentPrefix),
		Absent:   r.PostFormValue("absent") == "1",
		Comment:  web.FormValue(r, "comment"),
	}}

	if settings := h.school.PenaltySettings(); settings.Enabled {
		form.input.Done = formFields(r, entryDonePrefix)
		form.input.PenaltyFull = settings.Full
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

	settings := h.school.PenaltySettings()

	for i, workType := range entry.Types {
		row := view.MarksEntryRow{
			Name:         workType.Name,
			Field:        journal.EntryTypeKey(workType.ID),
			Value:        view.FormatMark(workType.Value),
			CommentField: entryCommentPrefix + strconv.FormatInt(workType.ID, 10),
			Comment:      workType.Comment,
			Autofocus:    i == 0,
		}

		if form.entered {
			row.Value = form.input.Values[workType.ID]
			row.Comment = form.input.Comments[workType.ID]
			row.Error = form.errs[row.Field]
		}

		if settings.Enabled && (workType.Penalty != nil || workType.CanPenalize) {
			row.Done = entryDone(settings, workType, form)
		}

		result.Rows = append(result.Rows, row)
	}

	if settings.Enabled && entry.Debt > 0 {
		result.Debt = settings.Name + " — долг: " + view.FormatPoints(settings.Unit, entry.Debt)
	}

	if len(result.Rows) == 0 {
		result.Empty, result.EmptyHref = entryEmpty(entry, user.ID)
	}

	if form.entered {
		result.Absent = form.input.Absent
		result.Comment = form.input.Comment
		result.CommentError = form.errs["comment"]
	}

	return result, nil
}

func entryDone(settings school.PenaltySettings, workType journal.EntryType, form entryForm) *view.DoneField {
	done := &view.DoneField{Field: journal.EntryDoneKey(workType.ID)}

	if penalty := workType.Penalty; penalty != nil {
		done.Value = strconv.Itoa(penalty.DonePercent)
		done.Points = view.FormatPoints(settings.Unit, penalty.Points)

		if penalty.Locked() {
			done.Locked = view.FormatPercent(penalty.DonePercent) + " · " + done.Points + " · " + web.PenaltyStatus(penalty.Status)
		}
	}

	if form.entered {
		done.Value = form.input.Done[workType.ID]
		done.Error = form.errs[done.Field]
	}

	return done
}

func entryEmpty(entry journal.MarksEntry, viewerID int64) (string, string) {
	switch {
	case !entry.InClass:
		return "Оценок за этот урок нет", ""
	case entry.Lesson.TeacherID == viewerID:
		return "Типов работ пока нет", workTypesBase(entry.Lesson.SubjectID)
	default:
		return msgAuthorWithoutTypes, ""
	}
}

func formFields(r *http.Request, prefix string) map[int64]string {
	fields := map[int64]string{}

	for key := range r.PostForm {
		raw, found := strings.CutPrefix(key, prefix)
		if !found {
			continue
		}

		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			fields[id] = web.FormValue(r, key)
		}
	}

	return fields
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
