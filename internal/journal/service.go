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
	marks         *storage.MarkRepo
	assignments   *storage.AssignmentRepo
	substitutions *storage.SubstitutionRepo
	classes       *storage.ClassRepo
	classStudents *storage.ClassStudentRepo
	subjects      *storage.SubjectRepo
	workTypes     *storage.WorkTypeRepo
	log           *slog.Logger
}

func NewService(
	lessons *storage.LessonRepo,
	marks *storage.MarkRepo,
	assignments *storage.AssignmentRepo,
	substitutions *storage.SubstitutionRepo,
	classes *storage.ClassRepo,
	classStudents *storage.ClassStudentRepo,
	subjects *storage.SubjectRepo,
	workTypes *storage.WorkTypeRepo,
	log *slog.Logger,
) *Service {
	return &Service{
		lessons:       lessons,
		marks:         marks,
		assignments:   assignments,
		substitutions: substitutions,
		classes:       classes,
		classStudents: classStudents,
		subjects:      subjects,
		workTypes:     workTypes,
		log:           log,
	}
}
