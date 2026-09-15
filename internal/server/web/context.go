package web

import (
	"context"
	"net/http"
	"slices"

	"github.com/ruskiiamov/school/internal/auth"
)

type contextKey int

const (
	userKey contextKey = iota
	requestInfoKey
)

type RequestInfo struct {
	UserID int64
}

func WithUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func UserFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(userKey).(auth.User)

	return user, ok
}

func WithRequestInfo(ctx context.Context, info *RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, info)
}

func RequestInfoFromContext(ctx context.Context) (*RequestInfo, bool) {
	info, ok := ctx.Value(requestInfoKey).(*RequestInfo)

	return info, ok
}

func (b *Base) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := b.Authenticate(r)
		if !ok {
			Redirect(w, r, "/login")
			return
		}

		if info, ok := RequestInfoFromContext(r.Context()); ok {
			info.UserID = user.ID
		}

		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

func RequireRole(roles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || !slices.Contains(roles, user.Role) {
				http.NotFound(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
