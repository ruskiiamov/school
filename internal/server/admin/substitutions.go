package admin

import (
	"errors"
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	substitutionsPath = "/admin/substitutions"
	classOption       = "Выберите класс"
)

type substitutionEdit struct {
	id      int64
	period  school.Period
	entered bool
	errs    validation.Errors
}

func (h *Handler) substitutionsList(w http.ResponseWriter, r *http.Request) {
	h.renderSubstitutions(w, r, view.SubstitutionFields{}, substitutionEdit{})
}

func (h *Handler) substitutionCreate(w http.ResponseWriter, r *http.Request) {
	input := school.SubstitutionInput{
		ClassID:   web.FormInt64(r, "class_id"),
		SubjectID: web.FormInt64(r, "subject_id"),
		TeacherID: web.FormInt64(r, "teacher_id"),
		StartDate: web.FormValue(r, "start_date"),
		EndDate:   web.FormValue(r, "end_date"),
	}

	err := h.createSubstitution(r, input)
	if errs, ok := web.FormErrors(err); ok {
		fields := view.SubstitutionFields{StartDate: input.StartDate, EndDate: input.EndDate, Errors: errs}
		h.renderSubstitutions(w, r, fields, substitutionEdit{})

		return
	}
	if err != nil {
		h.base.ServerError(w, r, "create substitution", err)
		return
	}

	h.substitutionsDone(w, r)
}

func (h *Handler) createSubstitution(r *http.Request, input school.SubstitutionInput) error {
	if input.TeacherID != 0 {
		_, err := h.activeUser(r.Context(), input.TeacherID, auth.RoleTeacher)
		if errors.Is(err, auth.ErrNotFound) {
			return validation.Errors{"teacher": msgTeacherUnknown}
		}
		if err != nil {
			return err
		}
	}

	_, err := h.school.CreateSubstitution(r.Context(), input)

	return err
}

func (h *Handler) substitutionUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	period := school.Period{StartDate: web.FormValue(r, "start_date"), EndDate: web.FormValue(r, "end_date")}

	err := h.school.UpdateSubstitutionPeriod(r.Context(), id, period)
	if errs, ok := web.FormErrors(err); ok {
		h.renderSubstitutions(w, r, view.SubstitutionFields{}, substitutionEdit{id: id, period: period, entered: true, errs: errs})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update substitution", err)
		return
	}

	h.substitutionsDone(w, r)
}

func (h *Handler) substitutionDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := h.school.DeleteSubstitution(r.Context(), id); err != nil {
		h.base.HandleServiceError(w, r, "delete substitution", err)
		return
	}

	h.substitutionsDone(w, r)
}

func (h *Handler) substitutionsDone(w http.ResponseWriter, r *http.Request) {
	if web.IsHTMX(r) {
		h.renderSubstitutions(w, r, view.SubstitutionFields{}, substitutionEdit{})
		return
	}

	web.Redirect(w, r, substitutionsListURL(r))
}

func showEnded(r *http.Request) bool {
	return r.URL.Query().Get("ended") == "1"
}

func substitutionsListURL(r *http.Request) string {
	if showEnded(r) {
		return substitutionsPath + "?ended=1"
	}

	return substitutionsPath
}

func (h *Handler) renderSubstitutions(w http.ResponseWriter, r *http.Request, newFields view.SubstitutionFields, edit substitutionEdit) {
	ctx := r.Context()
	ended := showEnded(r)

	substitutions, err := h.school.Substitutions(ctx, ended)
	if err != nil {
		h.base.ServerError(w, r, "list substitutions", err)
		return
	}

	classes, err := h.school.Classes(ctx, h.school.CurrentYear(), true)
	if err != nil {
		h.base.ServerError(w, r, "list classes", err)
		return
	}

	subjects, err := h.school.Subjects(ctx, true)
	if err != nil {
		h.base.ServerError(w, r, "list subjects", err)
		return
	}

	teachers, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleTeacher, IncludeInactive: true})
	if err != nil {
		h.base.ServerError(w, r, "list teachers", err)
		return
	}

	classByID := make(map[int64]school.Class, len(classes))
	for _, class := range classes {
		classByID[class.ID] = class
	}

	subjectByID := make(map[int64]school.Subject, len(subjects))
	for _, subject := range subjects {
		subjectByID[subject.ID] = subject
	}

	teacherByID := make(map[int64]auth.User, len(teachers))
	for _, teacher := range teachers {
		teacherByID[teacher.ID] = teacher
	}

	editing := editingID(r, edit.entered, edit.id)
	today := h.school.Today()

	rows := make([]view.SubstitutionRow, 0, len(substitutions))
	for _, substitution := range substitutions {
		subject := subjectByID[substitution.SubjectID]
		teacher := teacherByID[substitution.TeacherID]

		row := view.SubstitutionRow{
			ID:            substitution.ID,
			Class:         classByID[substitution.ClassID].Name,
			Subject:       subject.Name,
			Teacher:       teacher.FullName,
			SubjectActive: subject.Active,
			TeacherActive: teacher.Active,
			Period:        view.FormatPeriod(substitution.StartDate, substitution.EndDate),
			Ended:         substitution.Ended(today),
			Editing:       substitution.ID == editing,
		}

		if row.Editing {
			row.Fields.StartDate = substitution.StartDate.Format(validation.DateLayout)
			if !substitution.EndDate.IsZero() {
				row.Fields.EndDate = substitution.EndDate.Format(validation.DateLayout)
			}
		}

		if substitution.ID == edit.id {
			row.Fields.Errors = edit.errs
			if edit.entered {
				row.Fields.StartDate = edit.period.StartDate
				row.Fields.EndDate = edit.period.EndDate
			}
		}

		rows = append(rows, row)
	}

	newFields.Classes = []view.Option{{Name: classOption, Selected: true}}
	for _, class := range classes {
		if class.Active {
			newFields.Classes = append(newFields.Classes, view.Option{ID: class.ID, Name: class.Name})
		}
	}

	newFields.Subjects = []view.Option{{Name: subjectOption, Selected: true}}
	for _, subject := range subjects {
		if subject.Active {
			newFields.Subjects = append(newFields.Subjects, view.Option{ID: subject.ID, Name: subject.Name})
		}
	}

	newFields.Teachers = []view.Option{{Name: teacherOption, Selected: true}}
	for _, teacher := range teachers {
		if teacher.Active {
			newFields.Teachers = append(newFields.Teachers, view.Option{ID: teacher.ID, Name: teacher.FullName})
		}
	}

	page := view.SubstitutionsPage{
		Shell:         h.base.Shell(r, "Замены", substitutionsPath),
		Substitutions: rows,
		ShowEnded:     ended,
		CanCreate:     len(newFields.Classes) > 1 && len(newFields.Subjects) > 1 && len(newFields.Teachers) > 1,
		New:           newFields,
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.SubstitutionsList(page))
		return
	}

	h.base.Render(w, r, pages.Substitutions(page))
}
