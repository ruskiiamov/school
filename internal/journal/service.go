package journal

import (
	"errors"
	"log/slog"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type Service struct {
	location *time.Location
	log      *slog.Logger
}

func NewService(location *time.Location, log *slog.Logger) *Service {
	return &Service{location: location, log: log}
}
