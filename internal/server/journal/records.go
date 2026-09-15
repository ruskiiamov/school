package journal

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
)

type recordForm struct {
	entered bool
	input   journal.RecordInput
	errs    validation.Errors
}

func (h *Handler) recordSave(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	studentID, ok := web.PathValue(r, "sid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	user, _ := web.UserFromContext(r.Context())
	form := recordForm{entered: true, input: journal.RecordInput{
		Absent:  web.FormValue(r, "absent") == "1",
		Comment: web.FormValue(r, "comment"),
	}}

	err := h.journal.SaveRecord(r.Context(), user.ID, lesson.ID, studentID, form.input)
	if errs, ok := web.FormErrors(err); ok {
		form.errs = errs
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic, record: form}, h.markMode(r))

		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "save lesson record", err)
		return
	}

	h.marksDone(w, r, lesson)
}
