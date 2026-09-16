package server

import (
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user, _ := web.UserFromContext(r.Context())

	page := view.HomePage{
		Shell: s.base.Shell(r, "Дашборд", "/"),
		Today: view.FormatDate(time.Now().In(s.location)),
	}

	if user.Role != auth.RoleAdmin {
		page.Section = view.SectionItem(string(user.Role))
		page.SectionNote = sectionNote(user.Role)
		s.base.Render(w, r, pages.Home(page))

		return
	}

	stats, err := s.school.Stats(r.Context())
	if err != nil {
		s.base.ServerError(w, r, "load dashboard stats", err)
		return
	}

	teachers, err := s.auth.CountActiveUsers(r.Context(), auth.RoleTeacher)
	if err != nil {
		s.base.ServerError(w, r, "count teachers", err)
		return
	}

	page.YearName = school.YearName(stats.Year)
	page.NoClasses = stats.Classes == 0
	page.Stats = view.AdminStats(stats.Classes, stats.Students, teachers, stats.Subjects)

	s.base.Render(w, r, pages.Home(page))
}

func sectionNote(role auth.Role) string {
	if role == auth.RoleTeacher {
		return "Уроки, оценки, отсутствие и комментарии ученикам"
	}

	return "Уроки, оценки и комментарии по дням"
}
