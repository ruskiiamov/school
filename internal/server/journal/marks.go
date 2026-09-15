package journal

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
)

const (
	workTypeOption = "Тип работы"
	valueOption    = "Оценка"
)

type markForm struct {
	id      int64
	entered bool
	input   journal.MarkInput
	errs    validation.Errors
}

func (h *Handler) markAdd(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	form := markForm{entered: true, input: readMarkInput(r)}

	_, err := h.journal.AddMark(r.Context(), user.ID, lesson.ID, queryID(r, "student"), form.input)
	if errs, ok := web.FormErrors(err); ok {
		form.errs = errs
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic, mark: form}, h.markMode(r))

		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "add mark", err)
		return
	}

	h.marksDone(w, r, lesson)
}

func (h *Handler) markUpdate(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	markID, ok := web.PathValue(r, "mid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	user, _ := web.UserFromContext(r.Context())
	form := markForm{id: markID, entered: true, input: readMarkInput(r)}

	err := h.journal.UpdateMark(r.Context(), user.ID, lesson.ID, markID, form.input)
	if errs, ok := web.FormErrors(err); ok {
		form.errs = errs
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic, mark: form}, h.markMode(r))

		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update mark", err)
		return
	}

	h.marksDone(w, r, lesson)
}

func (h *Handler) markDelete(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.pathLesson(w, r)
	if !ok {
		return
	}

	markID, ok := web.PathValue(r, "mid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	user, _ := web.UserFromContext(r.Context())

	if err := h.journal.DeleteMark(r.Context(), user.ID, lesson.ID, markID); err != nil {
		h.base.HandleServiceError(w, r, "delete mark", err)
		return
	}

	h.marksDone(w, r, lesson)
}

func (h *Handler) markMode(r *http.Request) renderMode {
	if web.IsHTMX(r) {
		return renderBlock
	}

	return renderPage
}

func (h *Handler) marksDone(w http.ResponseWriter, r *http.Request, lesson journal.Lesson) {
	if web.IsHTMX(r) {
		h.renderLesson(w, r, lesson, lessonState{topic: lesson.Topic}, renderBlock)
		return
	}

	web.Redirect(w, r, studentURL(lesson.ID, queryID(r, "student"), 0))
}

func readMarkInput(r *http.Request) journal.MarkInput {
	return journal.MarkInput{
		WorkTypeID: web.FormInt64(r, "work_type_id"),
		Value:      int(web.FormInt64(r, "value")),
		Label:      web.FormValue(r, "label"),
	}
}

func (h *Handler) lessonBlock(r *http.Request, lesson journal.Lesson, form markForm) (view.LessonBlock, error) {
	ctx := r.Context()

	entries, err := h.journal.LessonStudents(ctx, lesson)
	if err != nil {
		return view.LessonBlock{}, err
	}

	students, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return view.LessonBlock{}, err
	}

	names := make(map[int64]auth.User, len(students))
	for _, student := range students {
		names[student.ID] = student
	}

	sort.Slice(entries, func(i, j int) bool {
		return names[entries[i].ID].FullName < names[entries[j].ID].FullName
	})

	if len(entries) == 0 {
		return view.LessonBlock{}, nil
	}

	selected := entries[0]
	requested := queryID(r, "student")

	for _, entry := range entries {
		if entry.ID == requested {
			selected = entry
		}
	}

	block := view.LessonBlock{Students: make([]view.LessonStudentItem, 0, len(entries))}

	for _, entry := range entries {
		user := names[entry.ID]
		block.Students = append(block.Students, view.LessonStudentItem{
			ID:        entry.ID,
			Href:      studentURL(lesson.ID, entry.ID, 0),
			FullName:  user.FullName,
			ShortName: view.ShortName(user.FullName),
			Summary:   markSummary(entry),
			Active:    user.Active,
			InClass:   entry.InClass,
			Selected:  entry.ID == selected.ID,
		})
	}

	panel, err := h.studentPanel(r, lesson, selected, names[selected.ID], form)
	if err != nil {
		return view.LessonBlock{}, err
	}

	block.Selected = &panel

	return block, nil
}

func (h *Handler) studentPanel(r *http.Request, lesson journal.Lesson, entry journal.StudentEntry, user auth.User, form markForm) (view.LessonStudentPanel, error) {
	workTypes, err := h.journal.ActiveWorkTypes(r.Context())
	if err != nil {
		return view.LessonStudentPanel{}, err
	}

	editing := queryID(r, "mark")
	if form.entered {
		editing = form.id
	}

	panel := view.LessonStudentPanel{
		ID:        entry.ID,
		FullName:  user.FullName,
		Active:    user.Active,
		InClass:   entry.InClass,
		AddAction: lessonPath(lesson.ID, "/marks") + studentQuery(entry.ID, 0),
		Error:     form.errs["student"],
	}

	for _, mark := range entry.Marks {
		row := view.MarkRow{
			ID:           mark.ID,
			WorkType:     mark.WorkTypeName,
			Value:        strconv.Itoa(mark.Value),
			Label:        mark.Label,
			Editing:      mark.ID == editing,
			Action:       lessonPath(lesson.ID, "/marks/"+strconv.FormatInt(mark.ID, 10)) + studentQuery(entry.ID, 0),
			EditHref:     studentURL(lesson.ID, entry.ID, mark.ID),
			CancelHref:   studentURL(lesson.ID, entry.ID, 0),
			DeleteAction: lessonPath(lesson.ID, "/marks/"+strconv.FormatInt(mark.ID, 10)+"/delete") + studentQuery(entry.ID, 0),
		}

		if row.Editing {
			input := journal.MarkInput{WorkTypeID: mark.WorkTypeID, Value: mark.Value, Label: mark.Label}
			errs := validation.Errors(nil)

			if form.entered && form.id == mark.ID {
				input, errs = form.input, form.errs
			}

			row.Fields = markFields(workTypes, input, errs)
		}

		panel.Marks = append(panel.Marks, row)
	}

	if form.entered && form.id == 0 {
		panel.New = markFields(workTypes, form.input, form.errs)
	} else {
		panel.New = markFields(workTypes, journal.MarkInput{}, nil)
	}

	return panel, nil
}

func markFields(workTypes []journal.WorkType, input journal.MarkInput, errs validation.Errors) view.MarkFields {
	fields := view.MarkFields{
		WorkTypes: []view.Option{{Name: workTypeOption, Selected: input.WorkTypeID == 0}},
		Values:    []view.Option{{Name: valueOption, Selected: input.Value == 0}},
		Label:     input.Label,
		Errors:    errs,
	}

	for _, workType := range workTypes {
		fields.WorkTypes = append(fields.WorkTypes, view.Option{ID: workType.ID, Name: workType.Name, Selected: workType.ID == input.WorkTypeID})
	}

	for value := 1; value <= 5; value++ {
		fields.Values = append(fields.Values, view.Option{ID: int64(value), Name: strconv.Itoa(value), Selected: value == input.Value})
	}

	return fields
}

func markSummary(entry journal.StudentEntry) string {
	values := make([]string, 0, len(entry.Marks))
	for _, mark := range entry.Marks {
		values = append(values, strconv.Itoa(mark.Value))
	}

	return strings.Join(values, ", ")
}

func studentQuery(studentID, markID int64) string {
	query := "?student=" + strconv.FormatInt(studentID, 10)
	if markID != 0 {
		query += "&mark=" + strconv.FormatInt(markID, 10)
	}

	return query
}

func studentURL(lessonID, studentID, markID int64) string {
	if studentID == 0 {
		return lessonPath(lessonID, "")
	}

	return lessonPath(lessonID, "") + studentQuery(studentID, markID)
}
