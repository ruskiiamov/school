package account

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const passwordPath = "/account/password"

func (h *Handler) passwordPage(w http.ResponseWriter, r *http.Request) {
	page := view.PasswordPage{
		Shell: h.base.Shell(r, "Смена пароля", passwordPath),
		Done:  r.URL.Query().Get("done") == "1",
	}

	h.base.Render(w, r, pages.AccountPassword(page))
}

func (h *Handler) passwordSubmit(w http.ResponseWriter, r *http.Request) {
	user, _ := web.UserFromContext(r.Context())

	sessionID := h.base.SessionID(r)

	values := map[string]string{
		"current": r.PostFormValue("current"),
		"new":     r.PostFormValue("new"),
		"repeat":  r.PostFormValue("repeat"),
	}

	err := h.auth.ChangePassword(r.Context(), user.ID, sessionID, auth.PasswordChange{
		Current: values["current"],
		New:     values["new"],
		Repeat:  values["repeat"],
	})
	if errs, ok := web.FormErrors(err); ok {
		page := view.PasswordPage{
			Shell:  h.base.Shell(r, "Смена пароля", passwordPath),
			Values: values,
			Errors: errs,
		}

		if web.IsHTMX(r) {
			h.base.Render(w, r, pages.PasswordErrors(page))
			return
		}

		h.base.Render(w, r, pages.AccountPassword(page))

		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "change password", err)
		return
	}

	web.Redirect(w, r, passwordPath+"?done=1")
}
