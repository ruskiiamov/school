package diary

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const classOption = "Класс"

type adminFilter struct {
	query   string
	year    int
	years   []view.Option
	classID int64
}

func (h *Handler) adminFilter(r *http.Request) (adminFilter, error) {
	year, years, err := web.YearSelection(r, h.school)
	if err != nil {
		return adminFilter{}, err
	}

	return adminFilter{
		query:   strings.TrimSpace(r.URL.Query().Get("q")),
		year:    year,
		years:   years,
		classID: queryID(r, "class"),
	}, nil
}

func (f adminFilter) values() url.Values {
	values := url.Values{}

	if f.query != "" {
		values.Set("q", f.query)
	}

	values.Set("year", strconv.Itoa(f.year))

	if f.classID != 0 {
		values.Set("class", strconv.FormatInt(f.classID, 10))
	}

	return values
}

func (f adminFilter) hidden() map[string]string {
	hidden := map[string]string{}
	for name, values := range f.values() {
		hidden[name] = values[0]
	}

	return hidden
}

func (f adminFilter) suffix() string {
	values := f.values()
	if len(values) == 0 {
		return ""
	}

	return "?" + values.Encode()
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter, err := h.adminFilter(r)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	classList, err := h.school.Classes(ctx, filter.year, false)
	if err != nil {
		h.base.ServerError(w, r, "list classes", err)
		return
	}

	page := view.DiarySearchPage{
		Shell:   h.base.Shell(r, "Дневники", adminDiaryPath),
		Path:    adminDiaryPath,
		Query:   filter.query,
		Years:   filter.years,
		Classes: []view.Option{{Name: classOption, Selected: filter.classID == 0}},
	}

	known := false

	for _, class := range classList {
		selected := class.ID == filter.classID
		known = known || selected
		page.Classes = append(page.Classes, view.Option{ID: class.ID, Name: class.Name, Selected: selected})
	}

	if !known {
		filter.classID = 0
	}

	page.Filtered = filter.query != "" || filter.classID != 0

	if page.Filtered {
		students, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, Query: filter.query})
		if err != nil {
			h.base.ServerError(w, r, "search students", err)
			return
		}

		classes, err := h.school.StudentClasses(ctx, filter.year)
		if err != nil {
			h.base.ServerError(w, r, "load student classes", err)
			return
		}

		for _, student := range students {
			class := classes[student.ID]
			if filter.classID != 0 && class.ID != filter.classID {
				continue
			}

			page.Students = append(page.Students, view.DiaryStudentRow{
				FullName:  student.FullName,
				ClassName: class.Name,
				Href:      adminDiaryPath + "/" + strconv.FormatInt(student.ID, 10) + filter.suffix(),
			})
		}
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.DiarySearchPage(page))
		return
	}

	h.base.Render(w, r, pages.DiarySearch(page))
}

func (h *Handler) studentDiary(w http.ResponseWriter, r *http.Request) {
	filter, err := h.adminFilter(r)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	student, title, ok := h.adminStudent(w, r, filter.year)
	if !ok {
		return
	}

	path := adminDiaryPath + "/" + strconv.FormatInt(student.ID, 10)

	h.renderDiary(w, r, diaryView{
		title:       "Дневник · " + title,
		active:      adminDiaryPath,
		path:        path,
		student:     student.ID,
		hidden:      filter.hidden(),
		backHref:    adminDiaryPath + filter.suffix(),
		summaryHref: path + "/summary" + filter.suffix(),
	})
}

func (h *Handler) adminStudent(w http.ResponseWriter, r *http.Request, year int) (auth.User, string, bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return auth.User{}, "", false
	}

	student, err := h.auth.UserByID(r.Context(), id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load student", err)
		return auth.User{}, "", false
	}

	if student.Role != auth.RoleStudent || !student.Active {
		http.NotFound(w, r)
		return auth.User{}, "", false
	}

	class, _, err := h.school.StudentClassIn(r.Context(), student.ID, year)
	if err != nil {
		h.base.ServerError(w, r, "load student class", err)
		return auth.User{}, "", false
	}

	title := student.FullName
	if class.Name != "" {
		title += " · " + class.Name
	}

	return student, title, true
}
