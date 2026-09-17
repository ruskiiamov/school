package admin

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const classesPath = "/admin/classes"

func (h *Handler) classesList(w http.ResponseWriter, r *http.Request) {
	h.renderClasses(w, r, "", "", rowEdit{})
}

func (h *Handler) classCreate(w http.ResponseWriter, r *http.Request) {
	name := web.FormValue(r, "name")

	_, err := h.school.CreateClass(r.Context(), name)
	if errs, ok := web.FormErrors(err); ok {
		h.renderClasses(w, r, name, errs["name"], rowEdit{})
		return
	}
	if err != nil {
		h.base.ServerError(w, r, "create class", err)
		return
	}

	h.classesDone(w, r)
}

func (h *Handler) classUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	name := web.FormValue(r, "name")

	err := h.school.UpdateClass(r.Context(), id, name)
	if errs, ok := web.FormErrors(err); ok {
		h.renderClasses(w, r, "", "", rowEdit{id: id, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "update class", err)
		return
	}

	h.classesDone(w, r)
}

func (h *Handler) classDeactivate(w http.ResponseWriter, r *http.Request) {
	h.classSetActive(w, r, false)
}

func (h *Handler) classActivate(w http.ResponseWriter, r *http.Request) {
	h.classSetActive(w, r, true)
}

func (h *Handler) classSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := h.school.SetClassActive(r.Context(), id, active)
	if errs, ok := web.FormErrors(err); ok {
		h.renderClasses(w, r, "", "", rowEdit{id: id, open: true, message: errs["name"]})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "set class active", err)
		return
	}

	h.classesDone(w, r)
}

func (h *Handler) classesDone(w http.ResponseWriter, r *http.Request) {
	if web.IsHTMX(r) {
		h.renderClasses(w, r, "", "", rowEdit{})
		return
	}

	years, err := h.school.ClassYears(r.Context())
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	web.Redirect(w, r, classesListURL(knownYear(years, h.queryYear(r), h.school.CurrentYear()), showInactive(r)))
}

func knownYear(years []int, requested, fallback int) int {
	for _, year := range years {
		if year == requested {
			return year
		}
	}

	return fallback
}

func (h *Handler) renderClasses(w http.ResponseWriter, r *http.Request, newName, newError string, edit rowEdit) {
	year := h.queryYear(r)
	inactive := showInactive(r)

	years, err := h.school.ClassYears(r.Context())
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	classes, err := h.school.Classes(r.Context(), year, inactive)
	if err != nil {
		h.base.ServerError(w, r, "list classes", err)
		return
	}

	sizes, err := h.school.ClassSizes(r.Context(), year)
	if err != nil {
		h.base.ServerError(w, r, "count class students", err)
		return
	}

	editing := editingID(r, edit.entered || edit.open, edit.id)

	rows := make([]view.ClassRow, 0, len(classes))
	for _, class := range classes {
		row := view.ClassRow{
			ID:       class.ID,
			Name:     class.Name,
			Href:     classPath(class.ID, ""),
			Active:   class.Active,
			Students: view.Plural(sizes[class.ID], "ученик", "ученика", "учеников"),
		}
		row.Editing = class.ID == editing

		if class.ID == edit.id {
			row.Error = edit.message
			if edit.entered {
				row.Name = edit.name
			}
		}

		rows = append(rows, row)
	}

	current := year == h.school.CurrentYear()

	transferHref := ""
	if current {
		canTransfer, err := h.school.CanTransfer(r.Context())
		if err != nil {
			h.base.ServerError(w, r, "check class transfer", err)
			return
		}

		if canTransfer {
			transferHref = transferPath
		}
	}

	page := view.ClassesPage{
		Shell:        h.base.Shell(r, "Классы", classesPath),
		Year:         year,
		YearName:     school.YearName(year),
		Years:        yearOptions(years, year, inactive),
		Classes:      rows,
		ShowInactive: inactive,
		CanCreate:    current,
		TransferHref: transferHref,
		ToggleHref:   classesListURL(year, !inactive),
		NewName:      newName,
		NewError:     newError,
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.ClassesList(page))
		return
	}

	h.base.Render(w, r, pages.Classes(page))
}

func yearOptions(years []int, selected int, inactive bool) []view.YearOption {
	options := make([]view.YearOption, 0, len(years))
	for _, year := range years {
		options = append(options, view.YearOption{
			Year:   year,
			Name:   school.YearName(year),
			Href:   classesListURL(year, inactive),
			Active: year == selected,
		})
	}

	return options
}

func (h *Handler) queryYear(r *http.Request) int {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		return h.school.CurrentYear()
	}

	return year
}

func classesListURL(year int, inactive bool) string {
	url := classesPath + "?year=" + strconv.Itoa(year)
	if inactive {
		url += "&inactive=1"
	}

	return url
}
