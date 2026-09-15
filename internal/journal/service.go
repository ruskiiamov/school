package journal

import (
	"errors"
	"log/slog"

	"github.com/ruskiiamov/school/internal/storage"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type Service struct {
	lessons       *storage.LessonRepo
	assignments   *storage.AssignmentRepo
	substitutions *storage.SubstitutionRepo
	classes       *storage.ClassRepo
	subjects      *storage.SubjectRepo
	log           *slog.Logger
}

func NewService(
	lessons *storage.LessonRepo,
	assignments *storage.AssignmentRepo,
	substitutions *storage.SubstitutionRepo,
	classes *storage.ClassRepo,
	subjects *storage.SubjectRepo,
	log *slog.Logger,
) *Service {
	return &Service{
		lessons:       lessons,
		assignments:   assignments,
		substitutions: substitutions,
		classes:       classes,
		subjects:      subjects,
		log:           log,
	}
}
