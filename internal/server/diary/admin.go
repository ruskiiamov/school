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

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	page := view.DiarySearchPage{
		Shell: h.base.Shell(r, "Дневники", adminDiaryPath),
		Path:  adminDiaryPath,
		Query: query,
	}

	if query != "" {
		students, err := h.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleStudent, Query: query})
		if err != nil {
			h.base.ServerError(w, r, "search students", err)
			return
		}

		classes, err := h.school.StudentClasses(r.Context(), h.school.CurrentYear())
		if err != nil {
			h.base.ServerError(w, r, "load student classes", err)
			return
		}

		search := "?" + url.Values{"q": {query}}.Encode()

		for _, student := range students {
			page.Students = append(page.Students, view.DiaryStudentRow{
				FullName:  student.FullName,
				ClassName: classes[student.ID].Name,
				Href:      adminDiaryPath + "/" + strconv.FormatInt(student.ID, 10) + search,
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
	student, title, ok := h.adminStudent(w, r)
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	path := adminDiaryPath + "/" + strconv.FormatInt(student.ID, 10)
	backHref := adminDiaryPath
	summaryHref := path + "/summary"
	hidden := map[string]string{}

	if query != "" {
		search := "?" + url.Values{"q": {query}}.Encode()
		backHref += search
		summaryHref += search
		hidden["q"] = query
	}

	h.renderDiary(w, r, diaryView{
		title:       "Дневник · " + title,
		active:      adminDiaryPath,
		path:        path,
		student:     student.ID,
		hidden:      hidden,
		backHref:    backHref,
		summaryHref: summaryHref,
	})
}

func (h *Handler) adminStudent(w http.ResponseWriter, r *http.Request) (auth.User, string, bool) {
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

	class, _, err := h.school.StudentClass(r.Context(), student.ID)
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
