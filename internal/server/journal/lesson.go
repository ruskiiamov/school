package journal

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

type lessonErrors struct {
	topic  string
	delete string
}

func (h *Handler) lessonShow(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	h.renderLesson(w, r, lesson, lesson.Topic, lessonErrors{})
}

func (h *Handler) lessonTopic(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	topic := web.FormValue(r, "topic")

	err := h.journal.UpdateTopic(r.Context(), user.ID, lesson.ID, topic)
	if errs, ok := web.FormErrors(err); ok {
		h.renderLesson(w, r, lesson, topic, lessonErrors{topic: errs["topic"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update lesson topic", err)
		return
	}

	if !web.IsHTMX(r) {
		web.Redirect(w, r, lessonPath(lesson.ID, ""))
		return
	}

	lesson.Topic = topic
	h.renderLesson(w, r, lesson, lesson.Topic, lessonErrors{})
}

func (h *Handler) lessonDelete(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())

	err := h.journal.DeleteLesson(r.Context(), user.ID, lesson.ID)
	if errs, ok := web.FormErrors(err); ok {
		h.renderLesson(w, r, lesson, lesson.Topic, lessonErrors{delete: errs["lesson"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "delete lesson", err)
		return
	}

	web.Redirect(w, r, journalPath)
}

func (h *Handler) pathLesson(w http.ResponseWriter, r *http.Request) (journal.Lesson, bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return journal.Lesson{}, false
	}

	user, _ := web.UserFromContext(r.Context())

	lesson, err := h.journal.LessonForTeacher(r.Context(), user.ID, id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load lesson", err)
		return journal.Lesson{}, false
	}

	return lesson, true
}

func (h *Handler) renderLesson(w http.ResponseWriter, r *http.Request, lesson journal.Lesson, topic string, errs lessonErrors) {
	canDelete, err := h.journal.CanDeleteLesson(r.Context(), lesson.ID)
	if err != nil {
		h.base.ServerError(w, r, "check lesson records", err)
		return
	}

	title := lesson.ClassName + " · " + lesson.SubjectName + " · " + view.FormatShortDate(lesson.Date)

	page := view.LessonPage{
		Shell:       h.base.Shell(r, title, journalPath),
		Title:       title,
		Path:        lessonPath(lesson.ID, ""),
		Topic:       topic,
		TopicError:  errs.topic,
		CanDelete:   canDelete,
		DeleteError: errs.delete,
	}

	if web.IsHTMX(r) && errs.delete == "" {
		h.base.Render(w, r, pages.LessonTopic(page))
		return
	}

	h.base.Render(w, r, pages.Lesson(page))
}
