package journal

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ruskiiamov/school/internal/files"
)

const strayFileAge = 24 * time.Hour

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

	var stray []string

	for _, id := range onDisk {
		if !rows[id] {
			stray = append(stray, id)
		}
	}

	if len(stray) > 0 && len(known) == 0 {
		s.log.WarnContext(ctx, "files on disk but no homework files in the database, nothing removed",
			slog.Int("files", len(stray)))

		return 0, nil
	}

	removed := 0
	cutoff := time.Now().Add(-strayFileAge)

	for _, id := range stray {
		if s.removeStray(ctx, id, cutoff) {
			removed++
		}
	}

	return removed, nil
}

func (s *Service) removeStray(ctx context.Context, id string, cutoff time.Time) bool {
	modified, err := s.store.ModTime(id)
	if errors.Is(err, files.ErrNotFound) {
		return false
	}
	if err != nil {
		s.log.ErrorContext(ctx, "check stray file age", slog.String("file_id", id), slog.Any("error", err))
		return false
	}

	if modified.After(cutoff) {
		return false
	}

	if err := s.store.Delete(id); err != nil && !errors.Is(err, files.ErrNotFound) {
		s.log.ErrorContext(ctx, "remove stray file", slog.String("file_id", id), slog.Any("error", err))
		return false
	}

	return true
}
