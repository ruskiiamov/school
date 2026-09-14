package server

import (
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	page := view.HomePage{
		Shell: s.shell(r, "Дашборд", "/"),
		Today: view.FormatDate(time.Now().In(s.location)),
	}

	if user.Role != auth.RoleAdmin {
		page.Section = view.SectionItem(string(user.Role))
		s.render(w, r, pages.Home(page))

		return
	}

	stats, err := s.school.Stats(r.Context())
	if err != nil {
		s.serverError(w, r, "load dashboard stats", err)
		return
	}

	teachers, err := s.auth.CountActiveUsers(r.Context(), auth.RoleTeacher)
	if err != nil {
		s.serverError(w, r, "count teachers", err)
		return
	}

	page.YearName = school.YearName(stats.Year)
	page.NoClasses = stats.Classes == 0
	page.Stats = view.AdminStats(stats.Classes, stats.Students, teachers, stats.Subjects)

	s.render(w, r, pages.Home(page))
}

func (s *Server) shell(r *http.Request, title, active string) view.Shell {
	user, _ := userFromContext(r.Context())

	return view.Shell{
		Title:      title + " — " + s.schoolName,
		SchoolName: s.schoolName,
		User:       toViewUser(user),
		Nav:        view.NavItems(string(user.Role), active),
	}
}

func toViewUser(user auth.User) view.User {
	return view.User{
		FullName: user.FullName,
		Role:     view.RoleTitle(string(user.Role)),
	}
}
