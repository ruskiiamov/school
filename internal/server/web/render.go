package web

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/view"
)

func (b *Base) Render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := component.Render(r.Context(), w); err != nil {
		b.log.ErrorContext(r.Context(), "render page",
			slog.Any("error", err),
			slog.String("path", r.URL.Path))
	}
}

func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)

		return
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (b *Base) Shell(r *http.Request, title, active string) view.Shell {
	user, _ := UserFromContext(r.Context())

	return view.Shell{
		Title:      title + " — " + b.schoolName,
		SchoolName: b.schoolName,
		User:       toViewUser(user),
		Nav:        view.NavItems(string(user.Role), active),
	}
}

func toViewUser(user auth.User) view.User {
	greeting := user.Name.First
	if user.Name.Middle != "" {
		greeting += " " + user.Name.Middle
	}

	if greeting == "" {
		greeting = user.FullName
	}

	return view.User{
		ShortName: view.ShortName(user.FullName),
		Greeting:  greeting,
		Role:      view.RoleTitle(string(user.Role)),
	}
}
