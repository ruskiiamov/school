package school

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ruskiiamov/school/internal/storage"
)

func (s *Service) Children(ctx context.Context) (map[int64][]int64, error) {
	links, err := s.parentChildren.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	children := make(map[int64][]int64, len(links))
	for _, link := range links {
		children[link.ParentID] = append(children[link.ParentID], link.StudentID)
	}

	return children, nil
}

func (s *Service) AddChild(ctx context.Context, parentID, studentID int64) error {
	if err := s.parentChildren.Add(ctx, parentID, studentID); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "child added to parent", slog.Int64("parent_id", parentID), slog.Int64("student_id", studentID))

	return nil
}

func (s *Service) RemoveChild(ctx context.Context, parentID, studentID int64) error {
	if err := s.parentChildren.Remove(ctx, parentID, studentID); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "child removed from parent", slog.Int64("parent_id", parentID), slog.Int64("student_id", studentID))

	return nil
}
