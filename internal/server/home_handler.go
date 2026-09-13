package server

import (
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		s.redirect(w, r, "/login")
		return
	}

	page := view.HomePage{
		Shell: view.Shell{
			Title:      "Дашборд — " + s.schoolName,
			SchoolName: s.schoolName,
			User:       toViewUser(user),
			Nav:        view.NavItems("/"),
		},
		Stats: view.PlaceholderStats(),
		Today: view.FormatDate(time.Now()),
	}

	s.render(w, r, pages.Home(page))
}

func toViewUser(user auth.User) view.User {
	return view.User{
		FullName: user.FullName,
		Role:     view.RoleTitle(string(user.Role)),
	}
}
