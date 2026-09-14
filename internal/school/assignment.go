package school

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgSubjectRequired = "Выберите предмет"
	msgSubjectUnknown  = "Такого предмета нет"
	msgTeacherRequired = "Выберите учителя"
)

type Assignment struct {
	ID        int64
	ClassID   int64
	SubjectID int64
	TeacherID int64
}

func (s *Service) Assignments(ctx context.Context, classID int64) ([]Assignment, error) {
	stored, err := s.assignments.ListByClass(ctx, classID)
	if err != nil {
		return nil, err
	}

	assignments := make([]Assignment, 0, len(stored))
	for _, assignment := range stored {
		assignments = append(assignments, Assignment(assignment))
	}

	return assignments, nil
}

func (s *Service) AssignTeacher(ctx context.Context, classID, subjectID, teacherID int64) error {
	if err := s.CheckStudentClass(ctx, classID); err != nil {
		return err
	}

	errs := validation.Errors{}

	if err := s.checkSubject(ctx, errs, subjectID); err != nil {
		return err
	}

	if teacherID == 0 {
		errs.Add("teacher", msgTeacherRequired)
	}

	if len(errs) > 0 {
		return errs
	}

	if err := s.assignments.Upsert(ctx, classID, subjectID, teacherID); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "teacher assigned",
		slog.Int64("class_id", classID), slog.Int64("subject_id", subjectID), slog.Int64("teacher_id", teacherID))

	return nil
}

func (s *Service) RemoveAssignment(ctx context.Context, classID, id int64) error {
	if err := s.CheckStudentClass(ctx, classID); err != nil {
		return err
	}

	if err := s.assignments.Remove(ctx, id, classID); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "assignment removed", slog.Int64("id", id), slog.Int64("class_id", classID))

	return nil
}

func (s *Service) checkSubject(ctx context.Context, errs validation.Errors, subjectID int64) error {
	if subjectID == 0 {
		errs.Add("subject", msgSubjectRequired)
		return nil
	}

	subject, err := s.SubjectByID(ctx, subjectID)
	if errors.Is(err, ErrNotFound) {
		errs.Add("subject", msgSubjectUnknown)
		return nil
	}
	if err != nil {
		return err
	}

	if !subject.Active {
		errs.Add("subject", msgSubjectUnknown)
	}

	return nil
}
