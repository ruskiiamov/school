package server

import (
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	page := view.HomePage{
		Shell: s.shell(r, "Дашборд", "/"),
		Stats: view.PlaceholderStats(),
		Today: view.FormatDate(time.Now().In(s.location)),
	}

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
