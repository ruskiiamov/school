package school

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgClassUnknown    = "Такого класса нет в текущем году"
	msgStudentHasClass = "Ученик уже в классе "
)

func (s *Service) StudentClass(ctx context.Context, studentID int64) (Class, bool, error) {
	return s.StudentClassIn(ctx, studentID, s.CurrentYear())
}

func (s *Service) StudentClassIn(ctx context.Context, studentID int64, year int) (Class, bool, error) {
	stored, err := s.classStudents.ClassOfStudent(ctx, studentID, year)
	if errors.Is(err, storage.ErrNotFound) {
		return Class{}, false, nil
	}
	if err != nil {
		return Class{}, false, err
	}

	return toClass(stored), true, nil
}

func (s *Service) StudentClasses(ctx context.Context, year int) (map[int64]Class, error) {
	members, err := s.classStudents.ListByYear(ctx, year)
	if err != nil {
		return nil, err
	}

	classes := make(map[int64]Class, len(members))
	for _, member := range members {
		classes[member.StudentID] = Class{ID: member.ClassID, Year: year, Name: member.ClassName, Active: true}
	}

	return classes, nil
}

func (s *Service) ClassSizes(ctx context.Context, year int) (map[int64]int, error) {
	members, err := s.classStudents.ListByYear(ctx, year)
	if err != nil {
		return nil, err
	}

	sizes := make(map[int64]int, len(members))
	for _, member := range members {
		sizes[member.ClassID]++
	}

	return sizes, nil
}

func (s *Service) CheckStudentClass(ctx context.Context, classID int64) error {
	if classID == 0 {
		return nil
	}

	class, err := s.ClassByID(ctx, classID)
	if errors.Is(err, ErrNotFound) {
		return validation.Errors{"class": msgClassUnknown}
	}
	if err != nil {
		return err
	}

	if !class.Active || class.Year != s.CurrentYear() {
		return validation.Errors{"class": msgClassUnknown}
	}

	return nil
}

func (s *Service) SetStudentClass(ctx context.Context, studentID, classID int64) error {
	if err := s.CheckStudentClass(ctx, classID); err != nil {
		return err
	}

	if err := s.classStudents.SetForYear(ctx, studentID, s.CurrentYear(), classID); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "student class set", slog.Int64("student_id", studentID), slog.Int64("class_id", classID))

	return nil
}

func (s *Service) AddClassStudent(ctx context.Context, classID, studentID int64) error {
	if err := s.CheckStudentClass(ctx, classID); err != nil {
		return err
	}

	current, err := s.classStudents.ClassOfStudent(ctx, studentID, s.CurrentYear())
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if err == nil {
		return validation.Errors{"student": msgStudentHasClass + current.Name}
	}

	if err := s.classStudents.Add(ctx, classID, studentID); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "student added to class", slog.Int64("student_id", studentID), slog.Int64("class_id", classID))

	return nil
}

func (s *Service) RemoveClassStudent(ctx context.Context, classID, studentID int64) error {
	if err := s.CheckStudentClass(ctx, classID); err != nil {
		return err
	}

	if err := s.classStudents.Remove(ctx, classID, studentID); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "student removed from class", slog.Int64("student_id", studentID), slog.Int64("class_id", classID))

	return nil
}
