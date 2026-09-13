package school

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

type Subject struct {
	ID     int64
	Name   string
	Active bool
}

type SubjectInput struct {
	Name string
}

func (s *Service) Subjects(ctx context.Context, includeInactive bool) ([]Subject, error) {
	stored, err := s.subjects.List(ctx, includeInactive)
	if err != nil {
		return nil, err
	}

	subjects := make([]Subject, 0, len(stored))
	for _, subject := range stored {
		subjects = append(subjects, toSubject(subject))
	}

	return subjects, nil
}

func (s *Service) SubjectByID(ctx context.Context, id int64) (Subject, error) {
	stored, err := s.subjects.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return Subject{}, ErrNotFound
	}
	if err != nil {
		return Subject{}, err
	}

	return toSubject(stored), nil
}

func (s *Service) CreateSubject(ctx context.Context, input SubjectInput) (int64, error) {
	name, err := s.validateSubject(ctx, 0, input)
	if err != nil {
		return 0, err
	}

	id, err := s.subjects.Create(ctx, name)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "subject created", slog.Int64("id", id), slog.String("name", name))

	return id, nil
}

func (s *Service) UpdateSubject(ctx context.Context, id int64, input SubjectInput) error {
	name, err := s.validateSubject(ctx, id, input)
	if err != nil {
		return err
	}

	if err := s.subjects.Update(ctx, id, name); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "subject updated", slog.Int64("id", id), slog.String("name", name))

	return nil
}

func (s *Service) SetSubjectActive(ctx context.Context, id int64, active bool) error {
	if active {
		subject, err := s.SubjectByID(ctx, id)
		if err != nil {
			return err
		}

		if err := s.requireSubjectNameFree(ctx, id, subject.Name); err != nil {
			return err
		}
	}

	if err := s.subjects.SetActive(ctx, id, active); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "subject active flag changed", slog.Int64("id", id), slog.Bool("active", active))

	return nil
}

func (s *Service) validateSubject(ctx context.Context, id int64, input SubjectInput) (string, error) {
	name := normalizeName(input.Name)

	errs := validation.Errors{}
	validateName(errs, name)

	if len(errs) > 0 {
		return "", errs
	}

	if err := s.requireSubjectNameFree(ctx, id, name); err != nil {
		return "", err
	}

	return name, nil
}

func (s *Service) requireSubjectNameFree(ctx context.Context, excludeID int64, name string) error {
	exists, err := s.subjects.ActiveNameExists(ctx, name, excludeID)
	if err != nil {
		return fmt.Errorf("check subject name: %w", err)
	}

	if exists {
		return validation.Errors{"name": msgNameTaken}
	}

	return nil
}

func toSubject(stored storage.Subject) Subject {
	return Subject{ID: stored.ID, Name: stored.Name, Active: stored.Active}
}
