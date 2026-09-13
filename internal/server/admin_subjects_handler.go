package server

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const subjectsPath = "/admin/subjects"

func (s *Server) subjectsList(w http.ResponseWriter, r *http.Request) {
	s.renderSubjects(w, r, "", "", rowEdit{})
}

func (s *Server) subjectCreate(w http.ResponseWriter, r *http.Request) {
	name := formValue(r, "name")

	_, err := s.school.CreateSubject(r.Context(), school.SubjectInput{Name: name})
	if errs, ok := formErrors(err); ok {
		s.renderSubjects(w, r, name, errs["name"], rowEdit{})
		return
	}
	if err != nil {
		s.serverError(w, r, "create subject", err)
		return
	}

	s.subjectsDone(w, r)
}

func (s *Server) subjectUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	name := formValue(r, "name")

	err := s.school.UpdateSubject(r.Context(), id, school.SubjectInput{Name: name})
	if errs, ok := formErrors(err); ok {
		s.renderSubjects(w, r, "", "", rowEdit{id: id, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "update subject", err)
		return
	}

	s.subjectsDone(w, r)
}

func (s *Server) subjectDeactivate(w http.ResponseWriter, r *http.Request) {
	s.subjectSetActive(w, r, false)
}

func (s *Server) subjectActivate(w http.ResponseWriter, r *http.Request) {
	s.subjectSetActive(w, r, true)
}

func (s *Server) subjectSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := s.school.SetSubjectActive(r.Context(), id, active)
	if errs, ok := formErrors(err); ok {
		s.renderSubjects(w, r, "", "", rowEdit{id: id, message: errs["name"]})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "set subject active", err)
		return
	}

	s.subjectsDone(w, r)
}

func (s *Server) subjectsDone(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		s.renderSubjects(w, r, "", "", rowEdit{})
		return
	}

	s.redirect(w, r, catalogListURL(subjectsPath, r))
}

func (s *Server) renderSubjects(w http.ResponseWriter, r *http.Request, newName, newError string, edit rowEdit) {
	inactive := showInactive(r)

	subjects, err := s.school.Subjects(r.Context(), inactive)
	if err != nil {
		s.serverError(w, r, "list subjects", err)
		return
	}

	editing := editingID(r, edit)

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
		Shell:        s.shell(r, "Предметы", subjectsPath),
		Subjects:     rows,
		ShowInactive: inactive,
		NewName:      newName,
		NewError:     newError,
	}

	if isHTMX(r) {
		s.render(w, r, pages.SubjectsList(page))
		return
	}

	s.render(w, r, pages.Subjects(page))
}
