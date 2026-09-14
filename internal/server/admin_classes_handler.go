package server

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const classesPath = "/admin/classes"

func (s *Server) classesList(w http.ResponseWriter, r *http.Request) {
	s.renderClasses(w, r, "", "", rowEdit{})
}

func (s *Server) classCreate(w http.ResponseWriter, r *http.Request) {
	name := formValue(r, "name")

	_, err := s.school.CreateClass(r.Context(), name)
	if errs, ok := formErrors(err); ok {
		s.renderClasses(w, r, name, errs["name"], rowEdit{})
		return
	}
	if err != nil {
		s.serverError(w, r, "create class", err)
		return
	}

	s.classesDone(w, r)
}

func (s *Server) classUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	name := formValue(r, "name")

	err := s.school.UpdateClass(r.Context(), id, name)
	if errs, ok := formErrors(err); ok {
		s.renderClasses(w, r, "", "", rowEdit{id: id, name: name, entered: true, message: errs["name"]})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "update class", err)
		return
	}

	s.classesDone(w, r)
}

func (s *Server) classDeactivate(w http.ResponseWriter, r *http.Request) {
	s.classSetActive(w, r, false)
}

func (s *Server) classActivate(w http.ResponseWriter, r *http.Request) {
	s.classSetActive(w, r, true)
}

func (s *Server) classSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := s.school.SetClassActive(r.Context(), id, active); err != nil {
		s.handleServiceError(w, r, "set class active", err)
		return
	}

	s.classesDone(w, r)
}

func (s *Server) classesDone(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		s.renderClasses(w, r, "", "", rowEdit{})
		return
	}

	years, err := s.school.ClassYears(r.Context())
	if err != nil {
		s.serverError(w, r, "list class years", err)
		return
	}

	s.redirect(w, r, classesListURL(knownYear(years, s.queryYear(r), s.school.CurrentYear()), showInactive(r)))
}

func knownYear(years []int, requested, fallback int) int {
	for _, year := range years {
		if year == requested {
			return year
		}
	}

	return fallback
}

func (s *Server) renderClasses(w http.ResponseWriter, r *http.Request, newName, newError string, edit rowEdit) {
	year := s.queryYear(r)
	inactive := showInactive(r)

	years, err := s.school.ClassYears(r.Context())
	if err != nil {
		s.serverError(w, r, "list class years", err)
		return
	}

	classes, err := s.school.Classes(r.Context(), year, inactive)
	if err != nil {
		s.serverError(w, r, "list classes", err)
		return
	}

	editing := editingID(r, edit.entered, edit.id)

	rows := make([]view.ClassRow, 0, len(classes))
	for _, class := range classes {
		row := view.ClassRow{
			ID:     class.ID,
			Name:   class.Name,
			Href:   classPath(class.ID, ""),
			Active: class.Active,
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

	page := view.ClassesPage{
		Shell:        s.shell(r, "Классы", classesPath),
		Year:         year,
		YearName:     school.YearName(year),
		Years:        yearOptions(years, year, inactive),
		Classes:      rows,
		ShowInactive: inactive,
		CanCreate:    year == s.school.CurrentYear(),
		ToggleHref:   classesListURL(year, !inactive),
		NewName:      newName,
		NewError:     newError,
	}

	if isHTMX(r) {
		s.render(w, r, pages.ClassesList(page))
		return
	}

	s.render(w, r, pages.Classes(page))
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

func (s *Server) queryYear(r *http.Request) int {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		return s.school.CurrentYear()
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
