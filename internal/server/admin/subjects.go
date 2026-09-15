package admin

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const subjectsPath = "/admin/subjects"

func (h *Handler) subjectsList(w http.ResponseWriter, r *http.Request) {
	h.renderSubjects(w, r, "", "", rowEdit{})
}

func (h *Handler) subjectCreate(w http.ResponseWriter, r *http.Request) {
	name := web.FormValue(r, "name")

	_, err := h.school.CreateSubject(r.Context(), school.SubjectInput{Name: name})
	if errs, ok := web.FormErrors(err); ok {
		h.renderSubjects(w, r, name, errs["name"], rowEdit{})
		return
	}
	if err != nil {
		h.base.ServerError(w, r, "create subject", err)
		return
	}

	h.subjectsDone(w, r)
}

func (h *Handler) subjectUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	name := web.FormValue(r, "name")

	err := h.school.UpdateSubject(r.Context(), id, school.SubjectInput{Name: name})
	if errs, ok := web.FormErrors(err); ok {
		h.renderSubjects(w, r, "", "", rowEdit{id: id, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update subject", err)
		return
	}

	h.subjectsDone(w, r)
}

func (h *Handler) subjectDeactivate(w http.ResponseWriter, r *http.Request) {
	h.subjectSetActive(w, r, false)
}

func (h *Handler) subjectActivate(w http.ResponseWriter, r *http.Request) {
	h.subjectSetActive(w, r, true)
}

func (h *Handler) subjectSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := h.school.SetSubjectActive(r.Context(), id, active)
	if errs, ok := web.FormErrors(err); ok {
		h.renderSubjects(w, r, "", "", rowEdit{id: id, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "set subject active", err)
		return
	}

	h.subjectsDone(w, r)
}

func (h *Handler) subjectsDone(w http.ResponseWriter, r *http.Request) {
	if web.IsHTMX(r) {
		h.renderSubjects(w, r, "", "", rowEdit{})
		return
	}

	web.Redirect(w, r, catalogListURL(subjectsPath, r))
}

func (h *Handler) renderSubjects(w http.ResponseWriter, r *http.Request, newName, newError string, edit rowEdit) {
	inactive := showInactive(r)

	subjects, err := h.school.Subjects(r.Context(), inactive)
	if err != nil {
		h.base.ServerError(w, r, "list subjects", err)
		return
	}

	editing := editingID(r, edit.entered, edit.id)

	rows := make([]view.SubjectRow, 0, len(subjects))
	for _, subject := range subjects {
		row := view.SubjectRow{ID: subject.ID, Name: subject.Name, Active: subject.Active}
		row.Editing = subject.ID == editing

		if subject.ID == edit.id {
			row.Error = edit.message
			if edit.entered {
				row.Name = edit.name
			}
		}

		rows = append(rows, row)
	}

	page := view.SubjectsPage{
		Shell:        h.base.Shell(r, "Предметы", subjectsPath),
		Subjects:     rows,
		ShowInactive: inactive,
		NewName:      newName,
		NewError:     newError,
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.SubjectsList(page))
		return
	}

	h.base.Render(w, r, pages.Subjects(page))
}
