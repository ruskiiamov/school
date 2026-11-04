package school

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ruskiiamov/school/internal/storage"
)

var ErrNotFound = errors.New("not found")

const maxNameLength = 100

type Service struct {
	yearStartMonth time.Month
	location       *time.Location
	subjects       *storage.SubjectRepo
	classes        *storage.ClassRepo
	classStudents  *storage.ClassStudentRepo
	parentChildren *storage.ParentChildRepo
	assignments    *storage.AssignmentRepo
	substitutions  *storage.SubstitutionRepo
	settings       *storage.SettingRepo
	log            *slog.Logger

	settingsMu sync.RWMutex
	penalty    PenaltySettings
}

func NewService(
	yearStartMonth time.Month,
	location *time.Location,
	subjects *storage.SubjectRepo,
	classes *storage.ClassRepo,
	classStudents *storage.ClassStudentRepo,
	parentChildren *storage.ParentChildRepo,
	assignments *storage.AssignmentRepo,
	substitutions *storage.SubstitutionRepo,
	settings *storage.SettingRepo,
	log *slog.Logger,
) *Service {
	return &Service{
		yearStartMonth: yearStartMonth,
		location:       location,
		subjects:       subjects,
		classes:        classes,
		classStudents:  classStudents,
		parentChildren: parentChildren,
		assignments:    assignments,
		substitutions:  substitutions,
		settings:       settings,
		log:            log,
		penalty:        DefaultPenaltySettings(),
	}
}
