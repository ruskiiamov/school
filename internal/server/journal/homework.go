package journal

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
)

const (
	msgFileRequired = "Выберите файл"
	msgUploadBroken = "Не удалось прочитать файл"

	uploadSlack = 1 << 20
)

type homeworkForm struct {
	entered   bool
	input     journal.HomeworkInput
	errs      validation.Errors
	fileError string
}

func (h *Handler) homeworkMode(r *http.Request) renderMode {
	if web.IsHTMX(r) {
		return renderHomework
	}

	return renderPage
}

func (h *Handler) homeworkSave(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	form := homeworkForm{entered: true, input: journal.HomeworkInput{
		Text: r.PostFormValue("text"),
		Due:  web.FormValue(r, "due"),
	}}

	err := h.journal.SaveHomework(r.Context(), user.ID, lesson.ID, form.input)
	if errs, ok := web.FormErrors(err); ok {
		form.errs = errs
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic, homework: form}, h.homeworkMode(r))

		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "save homework", err)
		return
	}

	h.homeworkDone(w, r, lesson)
}

func (h *Handler) homeworkUpload(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())

	if err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(h.files.TransferTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.base.ServerError(w, r, "extend read deadline", err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.files.MaxFileSize()*int64(h.files.MaxPerLesson)+uploadSlack)

	reader, err := r.MultipartReader()
	if err != nil {
		h.renderHomeworkError(w, r, lesson, msgFileRequired)
		return
	}

	uploaded := 0

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			h.renderHomeworkError(w, r, lesson, msgUploadBroken)
			return
		}

		if part.FormName() != "files" || part.FileName() == "" {
			continue
		}

		err = h.journal.AddHomeworkFile(r.Context(), user.ID, lesson.ID, part.FileName(), part)
		if errs, ok := web.FormErrors(err); ok {
			h.renderHomeworkError(w, r, lesson, errs["files"])
			return
		}
		if err != nil {
			h.base.HandleServiceError(w, r, "add homework file", err)
			return
		}

		uploaded++
	}

	if uploaded == 0 {
		h.renderHomeworkError(w, r, lesson, msgFileRequired)
		return
	}

	h.homeworkDone(w, r, lesson)
}

func (h *Handler) homeworkFileDelete(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())

	if err := h.journal.DeleteHomeworkFile(r.Context(), user.ID, lesson.ID, r.PathValue("fid")); err != nil {
		h.base.HandleServiceError(w, r, "delete homework file", err)
		return
	}

	h.homeworkDone(w, r, lesson)
}

func (h *Handler) renderHomeworkError(w http.ResponseWriter, r *http.Request, lesson journal.Lesson, message string) {
	h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic, homework: homeworkForm{fileError: message}}, h.homeworkMode(r))
}

func (h *Handler) homeworkDone(w http.ResponseWriter, r *http.Request, lesson journal.Lesson) {
	if web.IsHTMX(r) {
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic}, renderHomework)
		return
	}

	web.Redirect(w, r, lessonPath(lesson.ID, ""))
}

func (h *Handler) homeworkView(r *http.Request, lesson journal.Lesson, form homeworkForm, open bool) (view.LessonHomework, error) {
	homework, err := h.journal.Homework(r.Context(), lesson.ID)
	if err != nil {
		return view.LessonHomework{}, err
	}

	hw := view.LessonHomework{
		Open:         open,
		Status:       homeworkStatus(homework),
		Action:       lessonPath(lesson.ID, "/homework"),
		Text:         homework.Text,
		Errors:       form.errs,
		UploadAction: lessonPath(lesson.ID, "/homework/files"),
		CanUpload:    len(homework.Files) < h.files.MaxPerLesson,
		FileError:    form.fileError,
	}

	if !homework.Due.IsZero() {
		hw.Due = homework.Due.Format(validation.DateLayout)
	}

	if form.entered {
		hw.Text = form.input.Text
		hw.Due = form.input.Due
	}

	for _, file := range homework.Files {
		hw.Files = append(hw.Files, view.HomeworkFileRow{
			FileLink:     fileLink(file),
			DeleteAction: lessonPath(lesson.ID, "/homework/files/"+file.ID+"/delete"),
		})
	}

	return hw, nil
}

func homeworkStatus(homework journal.Homework) string {
	if homework.Empty() {
		return "не задано"
	}

	status := ""

	if !homework.Due.IsZero() {
		status = "к " + view.FormatShortDate(homework.Due)
	}

	if len(homework.Files) > 0 {
		if status != "" {
			status += " · "
		}

		status += view.Plural(len(homework.Files), "файл", "файла", "файлов")
	}

	if status == "" {
		return "задано"
	}

	return status
}

func fileLink(file journal.HomeworkFile) view.FileLink {
	return view.FileLink{Name: file.Name, Size: view.FormatFileSize(file.Size), Href: "/files/" + file.ID}
}
