package journal

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgWorkTypeNameRequired = "Укажите название"
	msgWorkTypeNameTooLong  = "Название длиннее 100 символов"
	msgWorkTypeNameTaken    = "Такой тип уже есть"
	maxWorkTypeNameLength   = 100
)

var defaultWorkTypes = []string{
	"Ответ на уроке",
	"Классная работа",
	"Самостоятельная работа",
	"Контрольная работа",
	"Домашняя работа (письменная)",
	"Домашняя работа (устная)",
}

type WorkType struct {
	ID        int64
	TeacherID int64
	SubjectID int64
	Name      string
	Active    bool
}

type Subject struct {
	ID   int64
	Name string
}

func (s *Service) TeacherSubjects(ctx context.Context, teacherID int64) ([]Subject, error) {
	stored, err := s.subjects.ListTaughtBy(ctx, teacherID)
	if err != nil {
		return nil, err
	}

	subjects := make([]Subject, 0, len(stored))
	for _, subject := range stored {
		subjects = append(subjects, Subject{ID: subject.ID, Name: subject.Name})
	}

	return subjects, nil
}

func (s *Service) TeacherSubject(ctx context.Context, teacherID, subjectID int64) (Subject, error) {
	subjects, err := s.TeacherSubjects(ctx, teacherID)
	if err != nil {
		return Subject{}, err
	}

	at := slices.IndexFunc(subjects, func(subject Subject) bool { return subject.ID == subjectID })
	if at < 0 {
		return Subject{}, ErrForbidden
	}

	return subjects[at], nil
}

func (s *Service) WorkTypes(ctx context.Context, teacherID, subjectID int64, includeInactive bool) ([]WorkType, error) {
	created, err := s.workTypes.CreateDefaults(ctx, teacherID, subjectID, defaultWorkTypes)
	if err != nil {
		return nil, err
	}

	if created {
		s.log.InfoContext(ctx, "default work types created", slog.Int64("teacher_id", teacherID), slog.Int64("subject_id", subjectID))
	}

	stored, err := s.workTypes.ListByOwner(ctx, teacherID, subjectID, includeInactive)
	if err != nil {
		return nil, err
	}

	workTypes := make([]WorkType, 0, len(stored))
	for _, workType := range stored {
		workTypes = append(workTypes, toWorkType(workType))
	}

	return workTypes, nil
}

func (s *Service) CreateWorkType(ctx context.Context, teacherID, subjectID int64, raw string) (int64, error) {
	name, err := s.validateWorkTypeName(ctx, teacherID, subjectID, 0, raw)
	if err != nil {
		return 0, err
	}

	sortOrder, err := s.workTypes.NextSortOrder(ctx, teacherID, subjectID)
	if err != nil {
		return 0, err
	}

	id, err := s.workTypes.Create(ctx, teacherID, subjectID, name, sortOrder)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "work type created", slog.Int64("id", id), slog.Int64("teacher_id", teacherID),
		slog.Int64("subject_id", subjectID), slog.String("name", name))

	return id, nil
}

func (s *Service) RenameWorkType(ctx context.Context, teacherID, id int64, raw string) error {
	workType, err := s.ownWorkType(ctx, teacherID, id)
	if err != nil {
		return err
	}

	name, err := s.validateWorkTypeName(ctx, teacherID, workType.SubjectID, id, raw)
	if err != nil {
		return err
	}

	if err := s.workTypes.Update(ctx, id, name); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "work type renamed", slog.Int64("id", id), slog.String("name", name))

	return nil
}

func (s *Service) SetWorkTypeActive(ctx context.Context, teacherID, id int64, active bool) error {
	workType, err := s.ownWorkType(ctx, teacherID, id)
	if err != nil {
		return err
	}

	if active {
		if err := s.requireWorkTypeNameFree(ctx, workType, id, workType.Name); err != nil {
			return err
		}
	}

	if err := s.workTypes.SetActive(ctx, id, active); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "work type active flag changed", slog.Int64("id", id), slog.Bool("active", active))

	return nil
}

func (s *Service) MoveWorkType(ctx context.Context, teacherID, id int64, step int) error {
	workType, err := s.ownWorkType(ctx, teacherID, id)
	if err != nil {
		return err
	}

	stored, err := s.workTypes.ListByOwner(ctx, teacherID, workType.SubjectID, true)
	if err != nil {
		return err
	}

	from := slices.IndexFunc(stored, func(item storage.WorkType) bool { return item.ID == id })
	if from < 0 || !stored[from].Active {
		return nil
	}

	to := from + step
	for to >= 0 && to < len(stored) && !stored[to].Active {
		to += step
	}

	if to < 0 || to >= len(stored) {
		return nil
	}

	stored[from], stored[to] = stored[to], stored[from]

	ids := make([]int64, 0, len(stored))
	for _, item := range stored {
		ids = append(ids, item.ID)
	}

	if err := s.workTypes.Reorder(ctx, ids); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "work type moved", slog.Int64("id", id), slog.Int("step", step))

	return nil
}

func (s *Service) OwnWorkType(ctx context.Context, teacherID, id int64) (WorkType, error) {
	workType, err := s.ownWorkType(ctx, teacherID, id)
	if err != nil {
		return WorkType{}, err
	}

	return toWorkType(workType), nil
}

func (s *Service) ownWorkType(ctx context.Context, teacherID, id int64) (storage.WorkType, error) {
	workType, err := s.workTypes.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return storage.WorkType{}, ErrNotFound
	}
	if err != nil {
		return storage.WorkType{}, err
	}

	if workType.TeacherID != teacherID {
		return storage.WorkType{}, ErrForbidden
	}

	return workType, nil
}

func (s *Service) validateWorkTypeName(ctx context.Context, teacherID, subjectID, id int64, raw string) (string, error) {
	name := validation.NormalizeSpaces(raw)

	switch {
	case name == "":
		return "", validation.Errors{"name": msgWorkTypeNameRequired}
	case utf8.RuneCountInString(name) > maxWorkTypeNameLength:
		return "", validation.Errors{"name": msgWorkTypeNameTooLong}
	}

	owner := storage.WorkType{TeacherID: teacherID, SubjectID: subjectID}
	if err := s.requireWorkTypeNameFree(ctx, owner, id, name); err != nil {
		return "", err
	}

	return name, nil
}

func (s *Service) requireWorkTypeNameFree(ctx context.Context, owner storage.WorkType, excludeID int64, name string) error {
	exists, err := s.workTypes.ActiveNameExists(ctx, owner.TeacherID, owner.SubjectID, name, excludeID)
	if err != nil {
		return err
	}

	if exists {
		return validation.Errors{"name": msgWorkTypeNameTaken}
	}

	return nil
}

func (s *Service) workTypeNames(ctx context.Context, ids []int64) (map[int64]string, map[int64]int, error) {
	stored, err := s.workTypes.ListByIDs(ctx, uniqueIDs(ids))
	if err != nil {
		return nil, nil, err
	}

	names := make(map[int64]string, len(stored))
	order := make(map[int64]int, len(stored))

	for i, workType := range stored {
		names[workType.ID] = workType.Name
		order[workType.ID] = i
	}

	return names, order, nil
}

func uniqueIDs(ids []int64) []int64 {
	unique := slices.Clone(ids)
	slices.Sort(unique)

	return slices.Compact(unique)
}

func toWorkType(stored storage.WorkType) WorkType {
	return WorkType{
		ID:        stored.ID,
		TeacherID: stored.TeacherID,
		SubjectID: stored.SubjectID,
		Name:      stored.Name,
		Active:    stored.Active,
	}
}
