package journal

import (
	"context"
	"errors"
	"log/slog"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/validation"
)

type LessonCardInput struct {
	Topic    string
	Homework HomeworkInput
	Remove   []string
}

func CheckLessonCard(input LessonCardInput) error {
	_, _, errs := parseHomework(input.Homework)

	if utf8.RuneCountInString(validation.NormalizeSpaces(input.Topic)) > maxTopicLength {
		errs.Add("topic", msgTopicTooLong)
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (s *Service) SaveLessonCard(ctx context.Context, teacherID, lessonID int64, input LessonCardInput) error {
	lesson, err := s.LessonForTeacher(ctx, teacherID, lessonID)
	if err != nil {
		return err
	}

	if err := CheckLessonCard(input); err != nil {
		return err
	}

	if topic := validation.NormalizeSpaces(input.Topic); topic != lesson.Topic {
		if err := s.UpdateTopic(ctx, teacherID, lessonID, topic); err != nil {
			return err
		}
	}

	if err := s.SaveHomework(ctx, teacherID, lessonID, input.Homework); err != nil {
		return err
	}

	for _, fileID := range input.Remove {
		if err := s.DeleteHomeworkFile(ctx, teacherID, lessonID, fileID); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}

	return nil
}

func (s *Service) DiscardHomeworkFiles(ctx context.Context, teacherID, lessonID int64, ids []string) {
	for _, id := range ids {
		if err := s.DeleteHomeworkFile(ctx, teacherID, lessonID, id); err != nil {
			s.log.ErrorContext(ctx, "discard homework file", slog.String("file_id", id), slog.Any("error", err))
		}
	}
}
