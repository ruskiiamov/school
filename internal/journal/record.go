package journal

import (
	"context"
	"log/slog"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgCommentTooLong = "Комментарий не длиннее 500 символов"

	maxCommentLength = 500
)

type RecordInput struct {
	Absent  bool
	Comment string
}

func validateRecord(input RecordInput) (RecordInput, error) {
	input.Comment = validation.NormalizeSpaces(input.Comment)
	if utf8.RuneCountInString(input.Comment) > maxCommentLength {
		return input, validation.Errors{"comment": msgCommentTooLong}
	}

	return input, nil
}

func (s *Service) storeRecord(ctx context.Context, lessonID, studentID int64, input RecordInput) error {
	if !input.Absent && input.Comment == "" {
		if err := s.lessonStudents.Delete(ctx, lessonID, studentID); err != nil {
			return err
		}

		s.log.InfoContext(ctx, "lesson record cleared", slog.Int64("lesson_id", lessonID), slog.Int64("student_id", studentID))

		return nil
	}

	err := s.lessonStudents.Upsert(ctx, storage.LessonStudent{
		LessonID:  lessonID,
		StudentID: studentID,
		Absent:    input.Absent,
		Comment:   input.Comment,
	})
	if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "lesson record saved", slog.Int64("lesson_id", lessonID),
		slog.Int64("student_id", studentID), slog.Bool("absent", input.Absent))

	return nil
}
