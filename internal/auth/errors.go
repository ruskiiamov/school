package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrNotFound           = errors.New("not found")
)
