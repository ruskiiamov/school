package journal

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	modalBodyID = "modal-body"
	closeModal  = "journal:close-modal"

	maxCardField = 64 << 10
)

type cardForm struct {
	entered bool
	input   journal.LessonCardInput
	errs    validation.Errors
}

func (h *Handler) lessonCard(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	h.renderCard(w, r, lesson, cardForm{})
}

func (h *Handler) lessonCardSave(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok || !h.prepareUpload(w, r) {
		return
	}

	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	user, _ := web.UserFromContext(r.Context())
	form := cardForm{entered: true}

	var added []string

	fail := func(errs validation.Errors) {
		h.journal.DiscardHomeworkFiles(r.Context(), user.ID, lesson.ID, added)
		form.errs = errs
		h.renderCard(w, r, lesson, form)
	}

	checked := false

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fail(validation.Errors{"files": msgUploadBroken})
			return
		}

		if part.FileName() == "" {
			if err := readCardField(part, &form.input); err != nil {
				fail(validation.Errors{"files": msgUploadBroken})
				return
			}

			continue
		}

		if part.FormName() != "files" {
			continue
		}

		if !checked {
			if errs, ok := web.FormErrors(journal.CheckLessonCard(form.input)); ok {
				fail(errs)
				return
			}

			checked = true
		}

		id, err := h.journal.AddHomeworkFile(r.Context(), user.ID, lesson.ID, part.FileName(), part)
		if errs, ok := web.FormErrors(err); ok {
			fail(errs)
			return
		}
		if err != nil {
			h.journal.DiscardHomeworkFiles(r.Context(), user.ID, lesson.ID, added)
			h.base.HandleServiceError(w, r, "add homework file", err)

			return
		}

		added = append(added, id)
	}

	err = h.journal.SaveLessonCard(r.Context(), user.ID, lesson.ID, form.input)
	if errs, ok := web.FormErrors(err); ok {
		fail(errs)
		return
	}
	if err != nil {
		h.journal.DiscardHomeworkFiles(r.Context(), user.ID, lesson.ID, added)
		h.base.HandleServiceError(w, r, "save lesson card", err)

		return
	}

	h.cardDone(w, r, lesson)
}

func readCardField(part *multipart.Part, input *journal.LessonCardInput) error {
	data, err := io.ReadAll(io.LimitReader(part, maxCardField))
	if err != nil {
		return err
	}

	value := string(data)

	switch part.FormName() {
	case "topic":
		input.Topic = strings.TrimSpace(value)
	case "text":
		input.Homework.Text = value
	case "due":
		input.Homework.Due = strings.TrimSpace(value)
	case "remove":
		input.Remove = append(input.Remove, value)
	}

	return nil
}

func (h *Handler) cardDone(w http.ResponseWriter, r *http.Request, lesson journal.Lesson) {
	if r.Header.Get("HX-Target") != modalBodyID {
		web.Redirect(w, r, pairPath(lesson.ClassID, lesson.SubjectID)+"#"+lessonAnchor(lesson.ID))
		return
	}

	content, _, err := h.pairContent(r, lesson.ClassID, lesson.SubjectID, pairState{highlight: lesson.ID})
	if err != nil {
		h.base.HandleServiceError(w, r, "load pair journal", err)
		return
	}

	w.Header().Set("HX-Trigger", closeModal)
	h.base.Render(w, r, pages.PairJournalContent(content, true))
}

func (h *Handler) renderCard(w http.ResponseWriter, r *http.Request, lesson journal.Lesson, form cardForm) {
	card, err := h.cardView(r, lesson, form)
	if err != nil {
		h.base.ServerError(w, r, "build lesson card", err)
		return
	}

	if !web.IsHTMX(r) {
		h.base.Render(w, r, pages.LessonCardPage(view.LessonCardPage{Shell: h.base.Shell(r, card.Title, homePath), Card: card}))
		return
	}

	w.Header().Set("HX-Push-Url", "false")
	h.base.Render(w, r, pages.LessonCard(card))
}

func (h *Handler) cardView(r *http.Request, lesson journal.Lesson, form cardForm) (view.LessonCard, error) {
	user, _ := web.UserFromContext(r.Context())

	homework, err := h.journal.Homework(r.Context(), lesson.ID)
	if err != nil {
		return view.LessonCard{}, err
	}

	card := view.LessonCard{
		Title:     "Урок " + view.FormatShortDate(lesson.Date),
		BackHref:  pairPath(lesson.ClassID, lesson.SubjectID) + "#" + lessonAnchor(lesson.ID),
		Action:    lessonPath(lesson.ID, "/card"),
		Topic:     lesson.Topic,
		Text:      homework.Text,
		Errors:    form.errs,
		CanUpload: len(homework.Files) < h.files.MaxPerLesson,
	}

	if !homework.Due.IsZero() {
		card.Due = homework.Due.Format(validation.InputDateLayout)
	}

	if form.entered {
		card.Topic = form.input.Topic
		card.Text = form.input.Homework.Text
		card.Due = form.input.Homework.Due
	}

	removed := map[string]bool{}
	for _, id := range form.input.Remove {
		removed[id] = true
	}

	for _, file := range homework.Files {
		card.Files = append(card.Files, view.LessonCardFile{FileLink: fileLink(file), ID: file.ID, Remove: removed[file.ID]})
	}

	if lesson.TeacherID != user.ID {
		names, err := h.teacherNames(r)
		if err != nil {
			return view.LessonCard{}, err
		}

		card.Author = "Урок провёл " + view.ShortName(names[lesson.TeacherID])
	}

	return card, nil
}

func studentMarksPath(lessonID, studentID int64) string {
	return lessonPath(lessonID, "/students/"+strconv.FormatInt(studentID, 10)+"/marks")
}
