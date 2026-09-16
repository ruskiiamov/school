package journal

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ruskiiamov/school/internal/files"
)

func (s *Service) RunCleanup(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	s.CleanupOrphans(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CleanupOrphans(ctx)
		}
	}
}

func (s *Service) CleanupOrphans(ctx context.Context) {
	marks, err := s.marks.DeleteOrphaned(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "delete orphaned marks", slog.Any("error", err))
		return
	}

	records, err := s.lessonStudents.DeleteOrphaned(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "delete orphaned lesson records", slog.Any("error", err))
		return
	}

	homework, err := s.homework.DeleteOrphaned(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "delete orphaned homework", slog.Any("error", err))
		return
	}

	orphanedFiles, err := s.homeworkFiles.DeleteOrphaned(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "delete orphaned homework files", slog.Any("error", err))
		return
	}

	for _, id := range orphanedFiles {
		if err := s.store.Delete(id); err != nil && !errors.Is(err, files.ErrNotFound) {
			s.log.ErrorContext(ctx, "remove orphaned homework file", slog.String("file_id", id), slog.Any("error", err))
		}
	}

	stray, err := s.cleanupDisk(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "clean up files directory", slog.Any("error", err))
		return
	}

	if marks+records+homework+int64(len(orphanedFiles))+int64(stray) > 0 {
		s.log.InfoContext(ctx, "journal orphans cleaned up", slog.Int64("marks", marks), slog.Int64("records", records),
			slog.Int64("homework", homework), slog.Int("homework_files", len(orphanedFiles)), slog.Int("stray_files", stray))
	}
}

func (s *Service) cleanupDisk(ctx context.Context) (int, error) {
	known, err := s.homeworkFiles.List(ctx)
	if err != nil {
		return 0, err
	}

	onDisk, err := s.store.IDs()
	if err != nil {
		return 0, err
	}

	present := make(map[string]bool, len(onDisk))
	for _, id := range onDisk {
		present[id] = true
	}

	rows := make(map[string]bool, len(known))

	for _, file := range known {
		rows[file.ID] = true

		if !present[file.ID] {
			s.log.WarnContext(ctx, "homework file missing on disk", slog.String("file_id", file.ID),
				slog.Int64("lesson_id", file.LessonID), slog.String("name", file.Name))
		}
	}

	removed := 0

	for _, id := range onDisk {
		if rows[id] {
			continue
		}

		if err := s.store.Delete(id); err != nil && !errors.Is(err, files.ErrNotFound) {
			s.log.ErrorContext(ctx, "remove stray file", slog.String("file_id", id), slog.Any("error", err))
			continue
		}

		removed++
	}

	return removed, nil
}
