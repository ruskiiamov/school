package journal

import (
	"context"
	"log/slog"
	"time"
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

	if marks+records > 0 {
		s.log.InfoContext(ctx, "journal orphans cleaned up", slog.Int64("marks", marks), slog.Int64("records", records))
	}
}
