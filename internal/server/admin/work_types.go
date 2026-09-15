package admin

import (
	"context"
	"net/http"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const workTypesPath = "/admin/work-types"

func (h *Handler) workTypesList(w http.ResponseWriter, r *http.Request) {
	h.renderWorkTypes(w, r, "", "", rowEdit{})
}

func (h *Handler) workTypeCreate(w http.ResponseWriter, r *http.Request) {
	name := web.FormValue(r, "name")

	_, err := h.school.CreateWorkType(r.Context(), school.WorkTypeInput{Name: name})
	if errs, ok := web.FormErrors(err); ok {
		h.renderWorkTypes(w, r, name, errs["name"], rowEdit{})
		return
	}
	if err != nil {
		h.base.ServerError(w, r, "create work type", err)
		return
	}

	h.workTypesDone(w, r)
}

func (h *Handler) workTypeUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	name := web.FormValue(r, "name")

	err := h.school.UpdateWorkType(r.Context(), id, school.WorkTypeInput{Name: name})
	if errs, ok := web.FormErrors(err); ok {
		h.renderWorkTypes(w, r, "", "", rowEdit{id: id, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update work type", err)
		return
	}

	h.workTypesDone(w, r)
}

func (h *Handler) workTypeDeactivate(w http.ResponseWriter, r *http.Request) {
	h.workTypeSetActive(w, r, false)
}

func (h *Handler) workTypeActivate(w http.ResponseWriter, r *http.Request) {
	h.workTypeSetActive(w, r, true)
}

func (h *Handler) workTypeSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := h.school.SetWorkTypeActive(r.Context(), id, active)
	if errs, ok := web.FormErrors(err); ok {
		h.renderWorkTypes(w, r, "", "", rowEdit{id: id, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "set work type active", err)
		return
	}

	h.workTypesDone(w, r)
}

func (h *Handler) workTypeUp(w http.ResponseWriter, r *http.Request) {
	h.workTypeMove(w, r, h.school.MoveWorkTypeUp)
}

func (h *Handler) workTypeDown(w http.ResponseWriter, r *http.Request) {
	h.workTypeMove(w, r, h.school.MoveWorkTypeDown)
}

func (h *Handler) workTypeMove(w http.ResponseWriter, r *http.Request, move func(context.Context, int64) error) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := move(r.Context(), id); err != nil {
		h.base.HandleServiceError(w, r, "move work type", err)
		return
	}

	h.workTypesDone(w, r)
}

func (h *Handler) workTypesDone(w http.ResponseWriter, r *http.Request) {
	if web.IsHTMX(r) {
		h.renderWorkTypes(w, r, "", "", rowEdit{})
		return
	}

	web.Redirect(w, r, catalogListURL(workTypesPath, r))
}

func (h *Handler) renderWorkTypes(w http.ResponseWriter, r *http.Request, newName, newError string, edit rowEdit) {
	inactive := showInactive(r)

	workTypes, err := h.school.WorkTypes(r.Context(), inactive)
	if err != nil {
		h.base.ServerError(w, r, "list work types", err)
		return
	}

	editing := editingID(r, edit.entered, edit.id)

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
		Shell:        h.base.Shell(r, "Типы работ", workTypesPath),
		WorkTypes:    rows,
		ShowInactive: inactive,
		NewName:      newName,
		NewError:     newError,
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.WorkTypesList(page))
		return
	}

	h.base.Render(w, r, pages.WorkTypes(page))
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
