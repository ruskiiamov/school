package school

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const msgClassNameTaken = "Такой класс в этом году уже есть"

type Class struct {
	ID     int64
	Year   int
	Name   string
	Active bool
}

func (s *Service) ClassYears(ctx context.Context) ([]int, error) {
	years, err := s.classes.Years(ctx)
	if err != nil {
		return nil, err
	}

	years = append(years, s.CurrentYear())
	slices.Sort(years)

	return slices.Compact(years), nil
}

func (s *Service) Classes(ctx context.Context, year int, includeInactive bool) ([]Class, error) {
	stored, err := s.classes.ListByYear(ctx, year, includeInactive)
	if err != nil {
		return nil, err
	}

	classes := make([]Class, 0, len(stored))
	for _, class := range stored {
		classes = append(classes, toClass(class))
	}

	return classes, nil
}

func (s *Service) ClassByID(ctx context.Context, id int64) (Class, error) {
	stored, err := s.classes.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return Class{}, ErrNotFound
	}
	if err != nil {
		return Class{}, err
	}

	return toClass(stored), nil
}

func (s *Service) CreateClass(ctx context.Context, name string) (int64, error) {
	name = validation.NormalizeSpaces(name)

	errs := validation.Errors{}
	validateName(errs, name)

	if len(errs) > 0 {
		return 0, errs
	}

	year := s.CurrentYear()

	if err := s.requireClassNameFree(ctx, year, name, 0); err != nil {
		return 0, err
	}

	id, err := s.classes.Create(ctx, year, name)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "class created", slog.Int64("id", id), slog.Int("year", year), slog.String("name", name))

	return id, nil
}

func (s *Service) UpdateClass(ctx context.Context, id int64, name string) error {
	class, err := s.ClassByID(ctx, id)
	if err != nil {
		return err
	}

	name = validation.NormalizeSpaces(name)

	errs := validation.Errors{}
	validateName(errs, name)

	if len(errs) > 0 {
		return errs
	}

	if err := s.requireClassNameFree(ctx, class.Year, name, id); err != nil {
		return err
	}

	if err := s.classes.Update(ctx, id, name); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "class updated", slog.Int64("id", id), slog.String("name", name))

	return nil
}

func (s *Service) SetClassActive(ctx context.Context, id int64, active bool) error {
	if err := s.classes.SetActive(ctx, id, active); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "class active flag changed", slog.Int64("id", id), slog.Bool("active", active))

	return nil
}

func (s *Service) requireClassNameFree(ctx context.Context, year int, name string, excludeID int64) error {
	exists, err := s.classes.NameExists(ctx, year, name, excludeID)
	if err != nil {
		return fmt.Errorf("check class name: %w", err)
	}

	if exists {
		return validation.Errors{"name": msgClassNameTaken}
	}

	return nil
}

func toClass(stored storage.Class) Class {
	return Class{ID: stored.ID, Year: stored.Year, Name: stored.Name, Active: stored.Active}
}
