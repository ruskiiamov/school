package server

import (
	"context"

	"github.com/ruskiiamov/school/internal/auth"
)

type contextKey int

const (
	userKey contextKey = iota
	requestIDKey
	requestInfoKey
)

type requestInfo struct {
	userID int64
}

func withUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func userFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(userKey).(auth.User)

	return user, ok
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)

	return id
}

func withRequestInfo(ctx context.Context, info *requestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, info)
}

func requestInfoFromContext(ctx context.Context) (*requestInfo, bool) {
	info, ok := ctx.Value(requestInfoKey).(*requestInfo)

	return info, ok
}
