package server

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const accountPasswordPath = "/account/password"

func (s *Server) passwordPage(w http.ResponseWriter, r *http.Request) {
	page := view.PasswordPage{
		Shell: s.shell(r, "Смена пароля", accountPasswordPath),
		Done:  r.URL.Query().Get("done") == "1",
	}

	s.render(w, r, pages.AccountPassword(page))
}

func (s *Server) passwordSubmit(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	sessionID := ""
	if cookie, err := r.Cookie(s.cookie.CookieName); err == nil {
		sessionID = cookie.Value
	}

	values := map[string]string{
		"current": r.PostFormValue("current"),
		"new":     r.PostFormValue("new"),
		"repeat":  r.PostFormValue("repeat"),
	}

	err := s.auth.ChangePassword(r.Context(), user.ID, sessionID, auth.PasswordChange{
		Current: values["current"],
		New:     values["new"],
		Repeat:  values["repeat"],
	})
	if errs, ok := formErrors(err); ok {
		page := view.PasswordPage{
			Shell:  s.shell(r, "Смена пароля", accountPasswordPath),
			Values: values,
			Errors: errs,
		}

		if isHTMX(r) {
			s.render(w, r, pages.PasswordErrors(page))
			return
		}

		s.render(w, r, pages.AccountPassword(page))

		return
	}
	if err != nil {
		s.handleServiceError(w, r, "change password", err)
		return
	}

	s.redirect(w, r, accountPasswordPath+"?done=1")
}
