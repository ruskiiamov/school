package admin

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	passwordResetPath = "/admin/password-reset"
	resetPageTitle    = "Сброс пароля"
	changedTitle      = "Пароль изменён"
	backToSearch      = "К поиску"
)

func (h *Handler) passwordResetList(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	page := view.PasswordResetPage{
		Shell: h.base.Shell(r, resetPageTitle, passwordResetPath),
		Path:  passwordResetPath,
		Query: query,
	}

	if query != "" {
		users, err := h.findUsers(r, query)
		if err != nil {
			h.base.ServerError(w, r, "search users", err)
			return
		}

		page.Users = users
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.PasswordResetPage(page))
		return
	}

	h.base.Render(w, r, pages.PasswordReset(page))
}

func (h *Handler) findUsers(r *http.Request, query string) ([]view.PasswordResetRow, error) {
	var users []auth.User

	for _, sec := range userSections {
		found, err := h.auth.Users(r.Context(), auth.UserFilter{Role: sec.role, Query: query})
		if err != nil {
			return nil, err
		}

		users = append(users, found...)
	}

	sort.SliceStable(users, func(i, j int) bool {
		return users[i].FullName < users[j].FullName
	})

	classes, err := h.school.StudentClasses(r.Context(), h.school.CurrentYear())
	if err != nil {
		return nil, err
	}

	search := encodeQuery(url.Values{"q": {query}})

	rows := make([]view.PasswordResetRow, 0, len(users))
	for _, user := range users {
		rows = append(rows, view.PasswordResetRow{
			ID:        user.ID,
			FullName:  user.FullName,
			Role:      view.RoleTitle(string(user.Role)),
			ClassName: classes[user.ID].Name,
			Login:     user.Login,
			Action:    passwordResetURL(user.ID, "") + search,
		})
	}

	return rows, nil
}

func (h *Handler) passwordReset(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resettableUser(w, r)
	if !ok {
		return
	}

	password, err := h.auth.ResetPassword(r.Context(), user.ID)
	if err != nil {
		h.base.HandleServiceError(w, r, "reset user password", err)
		return
	}

	h.created.put(h.base.SessionID(r), credentialsEntry{
		userID:   user.ID,
		kind:     credentialsPasswordChanged,
		login:    user.Login,
		password: password,
	})

	web.Redirect(w, r, passwordResetURL(user.ID, "/created")+searchQuery(r))
}

func (h *Handler) passwordResetCreated(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resettableUser(w, r)
	if !ok {
		return
	}

	backHref := passwordResetPath + searchQuery(r)

	entry, ok := h.created.take(h.base.SessionID(r), user.ID, credentialsPasswordChanged)
	if !ok {
		web.Redirect(w, r, backHref)
		return
	}

	page := view.UserCreatedPage{
		Shell:     h.base.Shell(r, changedTitle, passwordResetPath),
		Title:     changedTitle,
		FullName:  user.FullName,
		Login:     entry.login,
		Password:  entry.password,
		Note:      "Сессии пользователя сброшены. Новый пароль показан один раз, передайте его " + sectionByRole(user.Role).recipient,
		ListHref:  backHref,
		ListTitle: backToSearch,
	}

	h.base.Render(w, r, pages.UserCreated(page))
}

func (h *Handler) resettableUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return auth.User{}, false
	}

	user, err := h.auth.UserByID(r.Context(), id)
	if errors.Is(err, auth.ErrNotFound) || (err == nil && user.Role == auth.RoleAdmin) {
		http.NotFound(w, r)
		return auth.User{}, false
	}
	if err != nil {
		h.base.ServerError(w, r, "load user", err)
		return auth.User{}, false
	}

	return user, true
}

func sectionByRole(role auth.Role) userSection {
	for _, sec := range userSections {
		if sec.role == role {
			return sec
		}
	}

	return userSection{}
}

func passwordResetURL(id int64, suffix string) string {
	return passwordResetPath + "/" + strconv.FormatInt(id, 10) + suffix
}

func searchQuery(r *http.Request) string {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		return ""
	}

	return encodeQuery(url.Values{"q": {query}})
}
