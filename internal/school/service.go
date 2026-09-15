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
	classes        *storage.ClassRepo
	classStudents  *storage.ClassStudentRepo
	parentChildren *storage.ParentChildRepo
	assignments    *storage.AssignmentRepo
	substitutions  *storage.SubstitutionRepo
	log            *slog.Logger
}

func NewService(
	yearStartMonth time.Month,
	location *time.Location,
	subjects *storage.SubjectRepo,
	workTypes *storage.WorkTypeRepo,
	classes *storage.ClassRepo,
	classStudents *storage.ClassStudentRepo,
	parentChildren *storage.ParentChildRepo,
	assignments *storage.AssignmentRepo,
	substitutions *storage.SubstitutionRepo,
	log *slog.Logger,
) *Service {
	return &Service{
		yearStartMonth: yearStartMonth,
		location:       location,
		subjects:       subjects,
		workTypes:      workTypes,
		classes:        classes,
		classStudents:  classStudents,
		parentChildren: parentChildren,
		assignments:    assignments,
		substitutions:  substitutions,
		log:            log,
	}
}
