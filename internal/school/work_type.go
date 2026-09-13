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

type WorkType struct {
	ID        int64
	Name      string
	SortOrder int
	Active    bool
}

type WorkTypeInput struct {
	Name string
}

func (s *Service) WorkTypes(ctx context.Context, includeInactive bool) ([]WorkType, error) {
	stored, err := s.workTypes.List(ctx, includeInactive)
	if err != nil {
		return nil, err
	}

	workTypes := make([]WorkType, 0, len(stored))
	for _, workType := range stored {
		workTypes = append(workTypes, toWorkType(workType))
	}

	return workTypes, nil
}

func (s *Service) WorkTypeByID(ctx context.Context, id int64) (WorkType, error) {
	stored, err := s.workTypes.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return WorkType{}, ErrNotFound
	}
	if err != nil {
		return WorkType{}, err
	}

	return toWorkType(stored), nil
}

func (s *Service) CreateWorkType(ctx context.Context, input WorkTypeInput) (int64, error) {
	name, err := s.validateWorkTypeName(ctx, 0, input.Name)
	if err != nil {
		return 0, err
	}

	sortOrder, err := s.workTypes.NextSortOrder(ctx)
	if err != nil {
		return 0, err
	}

	id, err := s.workTypes.Create(ctx, name, sortOrder)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "work type created", slog.Int64("id", id), slog.String("name", name))

	return id, nil
}

func (s *Service) UpdateWorkType(ctx context.Context, id int64, input WorkTypeInput) error {
	name, err := s.validateWorkTypeName(ctx, id, input.Name)
	if err != nil {
		return err
	}

	if err := s.workTypes.Update(ctx, id, name); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "work type updated", slog.Int64("id", id), slog.String("name", name))

	return nil
}

func (s *Service) SetWorkTypeActive(ctx context.Context, id int64, active bool) error {
	if active {
		workType, err := s.WorkTypeByID(ctx, id)
		if err != nil {
			return err
		}

		if err := s.requireWorkTypeNameFree(ctx, id, workType.Name); err != nil {
			return err
		}
	}

	if err := s.workTypes.SetActive(ctx, id, active); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "work type active flag changed", slog.Int64("id", id), slog.Bool("active", active))

	return nil
}

func (s *Service) MoveWorkTypeUp(ctx context.Context, id int64) error {
	return s.moveWorkType(ctx, id, -1)
}

func (s *Service) MoveWorkTypeDown(ctx context.Context, id int64) error {
	return s.moveWorkType(ctx, id, 1)
}

func (s *Service) moveWorkType(ctx context.Context, id int64, step int) error {
	stored, err := s.workTypes.List(ctx, true)
	if err != nil {
		return err
	}

	from := slices.IndexFunc(stored, func(workType storage.WorkType) bool { return workType.ID == id })
	if from < 0 {
		return ErrNotFound
	}

	if !stored[from].Active {
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
	for _, workType := range stored {
		ids = append(ids, workType.ID)
	}

	if err := s.workTypes.Reorder(ctx, ids); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "work type moved", slog.Int64("id", id), slog.Int("step", step))

	return nil
}

func (s *Service) validateWorkTypeName(ctx context.Context, id int64, raw string) (string, error) {
	name := normalizeName(raw)

	errs := validation.Errors{}
	validateName(errs, name)

	if len(errs) > 0 {
		return "", errs
	}

	if err := s.requireWorkTypeNameFree(ctx, id, name); err != nil {
		return "", err
	}

	return name, nil
}

func (s *Service) requireWorkTypeNameFree(ctx context.Context, excludeID int64, name string) error {
	exists, err := s.workTypes.ActiveNameExists(ctx, name, excludeID)
	if err != nil {
		return fmt.Errorf("check work type name: %w", err)
	}

	if exists {
		return validation.Errors{"name": msgNameTaken}
	}

	return nil
}

func toWorkType(stored storage.WorkType) WorkType {
	return WorkType{ID: stored.ID, Name: stored.Name, SortOrder: stored.SortOrder, Active: stored.Active}
}
