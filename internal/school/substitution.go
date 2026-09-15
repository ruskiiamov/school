package school

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgClassRequired       = "Выберите класс"
	msgStartDateRequired   = "Укажите дату начала"
	msgDateInvalid         = "Неверная дата"
	msgEndBeforeStart      = "Дата окончания раньше начала"
	msgSubstitutionOverlap = "Период пересекается с другой заменой"
)

type Substitution struct {
	ID        int64
	ClassID   int64
	SubjectID int64
	TeacherID int64
	StartDate time.Time
	EndDate   time.Time
}

func (sub Substitution) Ended(today time.Time) bool {
	return !sub.EndDate.IsZero() && sub.EndDate.Before(today)
}

type SubstitutionInput struct {
	ClassID   int64
	SubjectID int64
	TeacherID int64
	StartDate string
	EndDate   string
}

type Period struct {
	StartDate string
	EndDate   string
}

func (s *Service) Substitutions(ctx context.Context, includeEnded bool) ([]Substitution, error) {
	stored, err := s.substitutions.ListByYear(ctx, s.CurrentYear(), s.Today(), includeEnded)
	if err != nil {
		return nil, err
	}

	substitutions := make([]Substitution, 0, len(stored))
	for _, substitution := range stored {
		substitutions = append(substitutions, toSubstitution(substitution))
	}

	return substitutions, nil
}

func (s *Service) SubstitutionByID(ctx context.Context, id int64) (Substitution, error) {
	stored, err := s.substitutions.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return Substitution{}, ErrNotFound
	}
	if err != nil {
		return Substitution{}, err
	}

	return toSubstitution(stored), nil
}

func (s *Service) CreateSubstitution(ctx context.Context, input SubstitutionInput) (int64, error) {
	errs := validation.Errors{}

	if input.ClassID == 0 {
		errs.Add("class", msgClassRequired)
	} else if err := s.checkSubstitutionClass(ctx, errs, input.ClassID); err != nil {
		return 0, err
	}

	if err := s.checkSubject(ctx, errs, input.SubjectID); err != nil {
		return 0, err
	}

	if input.TeacherID == 0 {
		errs.Add("teacher", msgTeacherRequired)
	}

	start, end := parsePeriod(errs, Period{StartDate: input.StartDate, EndDate: input.EndDate})

	if len(errs) > 0 {
		return 0, errs
	}

	if err := s.requireNoOverlap(ctx, input.ClassID, input.SubjectID, start, end, 0); err != nil {
		return 0, err
	}

	id, err := s.substitutions.Create(ctx, storage.Substitution{
		ClassID:   input.ClassID,
		SubjectID: input.SubjectID,
		TeacherID: input.TeacherID,
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "substitution created", slog.Int64("id", id),
		slog.Int64("class_id", input.ClassID), slog.Int64("subject_id", input.SubjectID), slog.Int64("teacher_id", input.TeacherID))

	return id, nil
}

func (s *Service) UpdateSubstitutionPeriod(ctx context.Context, id int64, period Period) error {
	substitution, err := s.SubstitutionByID(ctx, id)
	if err != nil {
		return err
	}

	errs := validation.Errors{}
	start, end := parsePeriod(errs, period)

	if len(errs) > 0 {
		return errs
	}

	if err := s.requireNoOverlap(ctx, substitution.ClassID, substitution.SubjectID, start, end, id); err != nil {
		return err
	}

	if err := s.substitutions.UpdatePeriod(ctx, id, start, end); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "substitution period updated", slog.Int64("id", id))

	return nil
}

func (s *Service) DeleteSubstitution(ctx context.Context, id int64) error {
	if err := s.substitutions.Delete(ctx, id); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "substitution deleted", slog.Int64("id", id))

	return nil
}

func (s *Service) checkSubstitutionClass(ctx context.Context, errs validation.Errors, classID int64) error {
	err := s.CheckStudentClass(ctx, classID)

	var classErrs validation.Errors
	if errors.As(err, &classErrs) {
		errs.Add("class", classErrs["class"])
		return nil
	}

	return err
}

func (s *Service) requireNoOverlap(ctx context.Context, classID, subjectID int64, start, end time.Time, excludeID int64) error {
	overlaps, err := s.substitutions.Overlaps(ctx, classID, subjectID, start, end, excludeID)
	if err != nil {
		return err
	}

	if overlaps {
		return validation.Errors{"start_date": msgSubstitutionOverlap}
	}

	return nil
}

func parsePeriod(errs validation.Errors, period Period) (time.Time, time.Time) {
	var start, end time.Time

	if period.StartDate == "" {
		errs.Add("start_date", msgStartDateRequired)
	} else {
		parsed, ok := validation.ParseDate(period.StartDate)
		if !ok {
			errs.Add("start_date", msgDateInvalid)
		}

		start = parsed
	}

	if period.EndDate != "" {
		parsed, ok := validation.ParseDate(period.EndDate)
		if !ok {
			errs.Add("end_date", msgDateInvalid)
		}

		end = parsed
	}

	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		errs.Add("end_date", msgEndBeforeStart)
	}

	return start, end
}

func toSubstitution(stored storage.Substitution) Substitution {
	return Substitution{
		ID:        stored.ID,
		ClassID:   stored.ClassID,
		SubjectID: stored.SubjectID,
		TeacherID: stored.TeacherID,
		StartDate: stored.StartDate,
		EndDate:   stored.EndDate,
	}
}
