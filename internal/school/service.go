package school

import (
	"errors"
	"log/slog"
	"time"
)

var ErrNotFound = errors.New("not found")

type Service struct {
	yearStartMonth time.Month
	location       *time.Location
	log            *slog.Logger
}

func NewService(yearStartMonth time.Month, location *time.Location, log *slog.Logger) *Service {
	return &Service{yearStartMonth: yearStartMonth, location: location, log: log}
}
