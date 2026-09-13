package school

import (
	"errors"
	"log/slog"
	"time"

	"github.com/ruskiiamov/school/internal/storage"
)

var ErrNotFound = errors.New("not found")

const maxNameLength = 100

type Service struct {
	yearStartMonth time.Month
	location       *time.Location
	subjects       *storage.SubjectRepo
	workTypes      *storage.WorkTypeRepo
	log            *slog.Logger
}

func NewService(
	yearStartMonth time.Month,
	location *time.Location,
	subjects *storage.SubjectRepo,
	workTypes *storage.WorkTypeRepo,
	log *slog.Logger,
) *Service {
	return &Service{
		yearStartMonth: yearStartMonth,
		location:       location,
		subjects:       subjects,
		workTypes:      workTypes,
		log:            log,
	}
}
