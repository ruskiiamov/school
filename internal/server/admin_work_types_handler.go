package server

import (
	"context"
	"net/http"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const workTypesPath = "/admin/work-types"

func (s *Server) workTypesList(w http.ResponseWriter, r *http.Request) {
	s.renderWorkTypes(w, r, "", "", rowEdit{})
}

func (s *Server) workTypeCreate(w http.ResponseWriter, r *http.Request) {
	name := formValue(r, "name")

	_, err := s.school.CreateWorkType(r.Context(), school.WorkTypeInput{Name: name})
	if errs, ok := formErrors(err); ok {
		s.renderWorkTypes(w, r, name, errs["name"], rowEdit{})
		return
	}
	if err != nil {
		s.serverError(w, r, "create work type", err)
		return
	}

	s.workTypesDone(w, r)
}

func (s *Server) workTypeUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	name := formValue(r, "name")

	err := s.school.UpdateWorkType(r.Context(), id, school.WorkTypeInput{Name: name})
	if errs, ok := formErrors(err); ok {
		s.renderWorkTypes(w, r, "", "", rowEdit{id: id, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "update work type", err)
		return
	}

	s.workTypesDone(w, r)
}

func (s *Server) workTypeDeactivate(w http.ResponseWriter, r *http.Request) {
	s.workTypeSetActive(w, r, false)
}

func (s *Server) workTypeActivate(w http.ResponseWriter, r *http.Request) {
	s.workTypeSetActive(w, r, true)
}

func (s *Server) workTypeSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := s.school.SetWorkTypeActive(r.Context(), id, active)
	if errs, ok := formErrors(err); ok {
		s.renderWorkTypes(w, r, "", "", rowEdit{id: id, message: errs["name"]})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "set work type active", err)
		return
	}

	s.workTypesDone(w, r)
}

func (s *Server) workTypeUp(w http.ResponseWriter, r *http.Request) {
	s.workTypeMove(w, r, s.school.MoveWorkTypeUp)
}

func (s *Server) workTypeDown(w http.ResponseWriter, r *http.Request) {
	s.workTypeMove(w, r, s.school.MoveWorkTypeDown)
}

func (s *Server) workTypeMove(w http.ResponseWriter, r *http.Request, move func(context.Context, int64) error) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := move(r.Context(), id); err != nil {
		s.handleServiceError(w, r, "move work type", err)
		return
	}

	s.workTypesDone(w, r)
}

func (s *Server) workTypesDone(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		s.renderWorkTypes(w, r, "", "", rowEdit{})
		return
	}

	s.redirect(w, r, catalogListURL(workTypesPath, r))
}

func (s *Server) renderWorkTypes(w http.ResponseWriter, r *http.Request, newName, newError string, edit rowEdit) {
	inactive := showInactive(r)

	workTypes, err := s.school.WorkTypes(r.Context(), inactive)
	if err != nil {
		s.serverError(w, r, "list work types", err)
		return
	}

	editing := editingID(r, edit)

	rows := make([]view.WorkTypeRow, 0, len(workTypes))
	for _, workType := range workTypes {
		row := view.WorkTypeRow{ID: workType.ID, Name: workType.Name, Active: workType.Active}
		row.Editing = workType.ID == editing

		if workType.ID == edit.id {
			row.Error = edit.message
			if edit.entered {
				row.Name = edit.name
			}
		}

		rows = append(rows, row)
	}

	markMoveBounds(rows)

	page := view.WorkTypesPage{
		Shell:        s.shell(r, "Типы работ", workTypesPath),
		WorkTypes:    rows,
		ShowInactive: inactive,
		NewName:      newName,
		NewError:     newError,
	}

	if isHTMX(r) {
		s.render(w, r, pages.WorkTypesList(page))
		return
	}

	s.render(w, r, pages.WorkTypes(page))
}

func markMoveBounds(rows []view.WorkTypeRow) {
	previous := -1

	for i := range rows {
		if !rows[i].Active {
			continue
		}

		if previous >= 0 {
			rows[i].CanUp = true
			rows[previous].CanDown = true
		}

		previous = i
	}
}
