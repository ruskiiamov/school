package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	invalidLoginMessage  = "Неверный логин или пароль"
	internalErrorMessage = "Не удалось войти, попробуйте позже"
)

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(r); ok {
		s.redirect(w, r, "/")
		return
	}

	s.render(w, r, pages.Login(view.LoginPage{SchoolName: s.schoolName}))
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	login := strings.TrimSpace(r.PostFormValue("login"))
	password := r.PostFormValue("password")

	session, err := s.auth.Login(r.Context(), login, password)
	if err != nil {
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			s.log.ErrorContext(r.Context(), "login failed", slog.Any("error", err))
			s.renderLoginError(w, r, login, internalErrorMessage)

			return
		}

		s.log.WarnContext(r.Context(), "invalid login attempt", slog.String("login", login))
		s.renderLoginError(w, r, login, invalidLoginMessage)

		return
	}

	s.setSessionCookie(w, session)
	s.log.InfoContext(r.Context(), "login succeeded", slog.Int64("user_id", session.UserID))
	s.redirect(w, r, "/")
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.cookie.CookieName); err == nil {
		if err := s.auth.Logout(r.Context(), cookie.Value); err != nil {
			s.log.ErrorContext(r.Context(), "logout failed", slog.Any("error", err))
		}
	}

	s.clearSessionCookie(w)
	s.redirect(w, r, "/login")
}

func (s *Server) renderLoginError(w http.ResponseWriter, r *http.Request, login, message string) {
	if r.Header.Get("HX-Request") == "true" {
		s.render(w, r, pages.LoginError(message))
		return
	}

	s.render(w, r, pages.Login(view.LoginPage{
		SchoolName: s.schoolName,
		Login:      login,
		Error:      message,
	}))
}
