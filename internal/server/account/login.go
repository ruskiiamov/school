package account

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	invalidLoginMessage  = "Неверный логин или пароль"
	tooManyMessage       = "Слишком много неудачных попыток. Попробуйте через 5 минут"
	internalErrorMessage = "Не удалось войти, попробуйте позже"
)

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.base.Authenticate(r); ok {
		web.Redirect(w, r, "/")
		return
	}

	h.base.Render(w, r, pages.Login(view.LoginPage{SchoolName: h.base.SchoolName()}))
}

func (h *Handler) loginSubmit(w http.ResponseWriter, r *http.Request) {
	login := strings.TrimSpace(r.PostFormValue("login"))
	password := r.PostFormValue("password")

	ip := h.base.ClientIP(r)
	device := h.base.DeviceToken(r)

	result, err := h.auth.LoginFrom(r.Context(), auth.LoginAttempt{Login: login, Password: password, DeviceToken: device, IP: ip})
	if errors.Is(err, auth.ErrTooManyAttempts) {
		h.log.WarnContext(r.Context(), "login blocked after repeated failures", slog.String("login", login), slog.String("ip", ip))
		h.renderLoginError(w, r, login, tooManyMessage)

		return
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		h.log.WarnContext(r.Context(), "invalid login attempt", slog.String("login", login), slog.String("ip", ip))
		h.renderLoginError(w, r, login, invalidLoginMessage)

		return
	}
	if err != nil {
		h.log.ErrorContext(r.Context(), "login failed", slog.Any("error", err))
		h.renderLoginError(w, r, login, internalErrorMessage)

		return
	}

	if !result.KnownDevice {
		device, err = h.auth.RememberDevice(r.Context(), result.Session.UserID)
		if err != nil {
			h.log.ErrorContext(r.Context(), "remember login device", slog.Any("error", err))
		}
	}

	if device != "" {
		h.base.SetDeviceCookie(w, device)
	}

	h.base.SetSessionCookie(w, result.Session)
	h.log.InfoContext(r.Context(), "login succeeded", slog.Int64("user_id", result.Session.UserID))
	web.Redirect(w, r, "/")
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if sessionID := h.base.SessionID(r); sessionID != "" {
		if err := h.auth.Logout(r.Context(), sessionID); err != nil {
			h.log.ErrorContext(r.Context(), "logout failed", slog.Any("error", err))
		}
	}

	h.base.ClearSessionCookie(w)
	web.Redirect(w, r, "/login")
}

func (h *Handler) renderLoginError(w http.ResponseWriter, r *http.Request, login, message string) {
	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.LoginError(message))
		return
	}

	h.base.Render(w, r, pages.Login(view.LoginPage{
		SchoolName: h.base.SchoolName(),
		Login:      login,
		Error:      message,
	}))
}
