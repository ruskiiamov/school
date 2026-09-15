package web

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/auth"
)

func (b *Base) Authenticate(r *http.Request) (auth.User, bool) {
	sessionID := b.SessionID(r)
	if sessionID == "" {
		return auth.User{}, false
	}

	user, err := b.auth.Authenticate(r.Context(), sessionID)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return auth.User{}, false
	}
	if err != nil {
		b.log.ErrorContext(r.Context(), "authenticate request", slog.Any("error", err))
		return auth.User{}, false
	}

	return user, true
}

func (b *Base) SessionID(r *http.Request) string {
	cookie, err := r.Cookie(b.cookie.CookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func (b *Base) SetSessionCookie(w http.ResponseWriter, session auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     b.cookie.CookieName,
		Value:    session.ID,
		Path:     "/",
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   b.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (b *Base) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     b.cookie.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   b.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
