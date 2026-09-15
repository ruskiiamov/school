package journal

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

type renderMode int

const (
	renderPage renderMode = iota
	renderTopic
	renderBlock
)

type lessonState struct {
	topic       string
	topicError  string
	deleteError string
	mark        markForm
	record      recordForm
}

func (h *Handler) lessonShow(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	mode := renderPage
	if web.IsHTMX(r) {
		mode = renderBlock
	}

	h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic}, mode)
}

func (h *Handler) lessonTopic(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	topic := web.FormValue(r, "topic")

	mode := renderPage
	if web.IsHTMX(r) {
		mode = renderTopic
	}

	err := h.journal.UpdateTopic(r.Context(), user.ID, lesson.ID, topic)
	if errs, ok := web.FormErrors(err); ok {
		h.renderLesson(w, r, lesson, lessonState{topic: topic, topicError: errs["topic"]}, mode)
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update lesson topic", err)
		return
	}

	if mode == renderPage {
		web.Redirect(w, r, lessonPath(lesson.ID, ""))
		return
	}

	lesson.Topic = topic
	h.renderLesson(w, r, lesson, lessonState{topic: topic}, mode)
}

func (h *Handler) lessonDelete(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())

	err := h.journal.DeleteLesson(r.Context(), user.ID, lesson.ID)
	if errs, ok := web.FormErrors(err); ok {
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic, deleteError: errs["lesson"]}, renderPage)
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

func (h *Handler) renderLesson(w http.ResponseWriter, r *http.Request, lesson journal.Lesson, state lessonState, mode renderMode) {
	title := lesson.ClassName + " · " + lesson.SubjectName + " · " + view.FormatShortDate(lesson.Date)

	page := view.LessonPage{
		Shell:       h.base.Shell(r, title, journalPath),
		Title:       title,
		Path:        lessonPath(lesson.ID, ""),
		Topic:       state.topic,
		TopicError:  state.topicError,
		DeleteError: state.deleteError,
	}

	if mode == renderTopic {
		h.base.Render(w, r, pages.LessonTopic(page))
		return
	}

	block, err := h.lessonBlock(r, lesson, state.mark, state.record)
	if err != nil {
		h.base.ServerError(w, r, "load lesson students", err)
		return
	}

	page.Block = block

	if mode == renderBlock {
		h.base.Render(w, r, pages.LessonBlock(block))
		return
	}

	canDelete, err := h.journal.CanDeleteLesson(r.Context(), lesson.ID)
	if err != nil {
		h.base.ServerError(w, r, "check lesson records", err)
		return
	}

	page.CanDelete = canDelete

	h.base.Render(w, r, pages.Lesson(page))
}

func queryID(r *http.Request, name string) int64 {
	id, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}

	return id
}
