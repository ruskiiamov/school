package journal

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const workTypesPath = journalPath + "/work-types"

type workTypeEdit struct {
	id      int64
	name    string
	entered bool
	open    bool
	message string
}

func workTypesBase(subjectID int64) string {
	return journalPath + "/subjects/" + strconv.FormatInt(subjectID, 10) + "/work-types"
}

func (h *Handler) workTypesIndex(w http.ResponseWriter, r *http.Request) {
	user, _ := web.UserFromContext(r.Context())

	subjects, err := h.journal.TeacherSubjects(r.Context(), user.ID)
	if err != nil {
		h.base.ServerError(w, r, "list teacher subjects", err)
		return
	}

	if len(subjects) > 0 {
		web.Redirect(w, r, workTypesBase(subjects[0].ID))
		return
	}

	h.base.Render(w, r, pages.WorkTypes(view.WorkTypesPage{Shell: h.base.Shell(r, "Типы работ", workTypesPath)}))
}

func (h *Handler) workTypesList(w http.ResponseWriter, r *http.Request) {
	subjectID, ok := web.PathValue(r, "subject")
	if !ok {
		http.NotFound(w, r)
		return
	}

	h.renderWorkTypes(w, r, subjectID, "", "", workTypeEdit{})
}

func (h *Handler) workTypeCreate(w http.ResponseWriter, r *http.Request) {
	subjectID, ok := web.PathValue(r, "subject")
	if !ok {
		http.NotFound(w, r)
		return
	}

	user, _ := web.UserFromContext(r.Context())

	if _, err := h.journal.TeacherSubject(r.Context(), user.ID, subjectID); err != nil {
		h.base.HandleServiceError(w, r, "load teacher subject", err)
		return
	}

	name := web.FormValue(r, "name")

	_, err := h.journal.CreateWorkType(r.Context(), user.ID, subjectID, name)
	if errs, ok := web.FormErrors(err); ok {
		h.renderWorkTypes(w, r, subjectID, name, errs["name"], workTypeEdit{})
		return
	}
	if err != nil {
		h.base.ServerError(w, r, "create work type", err)
		return
	}

	h.workTypesDone(w, r, subjectID)
}

func (h *Handler) workTypeUpdate(w http.ResponseWriter, r *http.Request) {
	workType, ok := h.pathWorkType(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	name := web.FormValue(r, "name")

	err := h.journal.RenameWorkType(r.Context(), user.ID, workType.ID, name)
	if errs, ok := web.FormErrors(err); ok {
		h.renderWorkTypes(w, r, workType.SubjectID, "", "", workTypeEdit{id: workType.ID, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "rename work type", err)
		return
	}

	h.workTypesDone(w, r, workType.SubjectID)
}

func (h *Handler) workTypeDeactivate(w http.ResponseWriter, r *http.Request) {
	h.workTypeSetActive(w, r, false)
}

func (h *Handler) workTypeActivate(w http.ResponseWriter, r *http.Request) {
	h.workTypeSetActive(w, r, true)
}

func (h *Handler) workTypeSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	workType, ok := h.pathWorkType(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())

	err := h.journal.SetWorkTypeActive(r.Context(), user.ID, workType.ID, active)
	if errs, ok := web.FormErrors(err); ok {
		h.renderWorkTypes(w, r, workType.SubjectID, "", "", workTypeEdit{id: workType.ID, open: true, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "set work type active", err)
		return
	}

	h.workTypesDone(w, r, workType.SubjectID)
}

func (h *Handler) workTypeUp(w http.ResponseWriter, r *http.Request) {
	h.workTypeMove(w, r, -1)
}

func (h *Handler) workTypeDown(w http.ResponseWriter, r *http.Request) {
	h.workTypeMove(w, r, 1)
}

func (h *Handler) workTypeMove(w http.ResponseWriter, r *http.Request, step int) {
	workType, ok := h.pathWorkType(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())

	if err := h.journal.MoveWorkType(r.Context(), user.ID, workType.ID, step); err != nil {
		h.base.HandleServiceError(w, r, "move work type", err)
		return
	}

	h.workTypesDone(w, r, workType.SubjectID)
}

func (h *Handler) pathWorkType(w http.ResponseWriter, r *http.Request) (journal.WorkType, bool) {
	subjectID, subjectOK := web.PathValue(r, "subject")
	id, idOK := web.PathID(r)

	if !subjectOK || !idOK {
		http.NotFound(w, r)
		return journal.WorkType{}, false
	}

	user, _ := web.UserFromContext(r.Context())

	workType, err := h.journal.OwnWorkType(r.Context(), user.ID, id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load work type", err)
		return journal.WorkType{}, false
	}

	if workType.SubjectID != subjectID {
		http.NotFound(w, r)
		return journal.WorkType{}, false
	}

	return workType, true
}

func (h *Handler) workTypesDone(w http.ResponseWriter, r *http.Request, subjectID int64) {
	if web.IsHTMX(r) {
		h.renderWorkTypes(w, r, subjectID, "", "", workTypeEdit{})
		return
	}

	target := workTypesBase(subjectID)
	if showInactive(r) {
		target += "?inactive=1"
	}

	web.Redirect(w, r, target)
}

func (h *Handler) renderWorkTypes(w http.ResponseWriter, r *http.Request, subjectID int64, newName, newError string, edit workTypeEdit) {
	user, _ := web.UserFromContext(r.Context())

	subjects, err := h.journal.TeacherSubjects(r.Context(), user.ID)
	if err != nil {
		h.base.ServerError(w, r, "list teacher subjects", err)
		return
	}

	page := view.WorkTypesPage{
		Shell:        h.base.Shell(r, "Типы работ", workTypesPath),
		Base:         workTypesBase(subjectID),
		ShowInactive: showInactive(r),
		NewName:      newName,
		NewError:     newError,
	}

	found := false

	for _, subject := range subjects {
		active := subject.ID == subjectID
		found = found || active
		page.Subjects = append(page.Subjects, view.SubjectTab{Name: subject.Name, Href: workTypesBase(subject.ID), Active: active})
	}

	if !found {
		http.NotFound(w, r)
		return
	}

	workTypes, err := h.journal.WorkTypes(r.Context(), user.ID, subjectID, page.ShowInactive)
	if err != nil {
		h.base.ServerError(w, r, "list work types", err)
		return
	}

	page.WorkTypes = workTypeRows(workTypes, editingWorkType(r, edit), edit)

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.WorkTypesList(page))
		return
	}

	h.base.Render(w, r, pages.WorkTypes(page))
}

func workTypeRows(workTypes []journal.WorkType, editing int64, edit workTypeEdit) []view.WorkTypeRow {
	rows := make([]view.WorkTypeRow, 0, len(workTypes))

	for _, workType := range workTypes {
		row := view.WorkTypeRow{ID: workType.ID, Name: workType.Name, Active: workType.Active, Editing: workType.ID == editing}

		if workType.ID == edit.id {
			row.Error = edit.message
			if edit.entered {
				row.Name = edit.name
			}
		}

		rows = append(rows, row)
	}

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

	return rows
}

func editingWorkType(r *http.Request, edit workTypeEdit) int64 {
	if edit.entered || edit.open {
		return edit.id
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("edit"), 10, 64)
	if err != nil {
		return 0
	}

	return id
}

func showInactive(r *http.Request) bool {
	return r.URL.Query().Get("inactive") == "1"
}
