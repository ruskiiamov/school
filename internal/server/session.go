package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
)

func (s *Server) authenticate(r *http.Request) (auth.User, bool) {
	cookie, err := r.Cookie(s.cookie.CookieName)
	if err != nil {
		return auth.User{}, false
	}

	user, err := s.auth.Authenticate(r.Context(), cookie.Value)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return auth.User{}, false
	}
	if err != nil {
		s.log.ErrorContext(r.Context(), "authenticate request", slog.Any("error", err))
		return auth.User{}, false
	}

	return user, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, session auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.CookieName,
		Value:    session.ID,
		Path:     "/",
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   s.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
