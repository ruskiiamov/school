package journal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const msgLessonExists = "Урок на %s уже есть"

type YearBounds struct {
	Start time.Time
	End   time.Time
}

func (s *Service) CreateLesson(ctx context.Context, teacherID, classID, subjectID int64, date string, bounds YearBounds) (int64, error) {
	class, err := s.classes.ByID(ctx, classID)
	if errors.Is(err, storage.ErrNotFound) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	if _, err := s.yearPair(ctx, teacherID, class.Year, classID, subjectID); err != nil {
		return 0, err
	}

	parsed, err := s.checkLessonDate(ctx, teacherID, classID, subjectID, class.Year, date, bounds)
	if err != nil {
		return 0, err
	}

	id, err := s.lessons.Create(ctx, storage.Lesson{ClassID: classID, SubjectID: subjectID, TeacherID: teacherID, Date: parsed})
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "lesson created", slog.Int64("id", id), slog.Int64("teacher_id", teacherID),
		slog.Int64("class_id", classID), slog.Int64("subject_id", subjectID), slog.String("date", date))

	return id, nil
}

func (s *Service) MoveLesson(ctx context.Context, teacherID, lessonID int64, date string, bounds YearBounds) error {
	lesson, err := s.LessonForTeacher(ctx, teacherID, lessonID)
	if err != nil {
		return err
	}

	if parsed, ok := parseLessonDate(date, bounds); ok && parsed.Equal(lesson.Date) {
		return nil
	}

	parsed, err := s.checkLessonDate(ctx, teacherID, lesson.ClassID, lesson.SubjectID, lesson.Year, date, bounds)
	if err != nil {
		return err
	}

	if err := s.lessons.UpdateDate(ctx, lessonID, parsed); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "lesson moved", slog.Int64("id", lessonID), slog.Int64("teacher_id", teacherID), slog.String("date", date))

	return nil
}

func (s *Service) checkLessonDate(ctx context.Context, teacherID, classID, subjectID int64, year int, date string, bounds YearBounds) (time.Time, error) {
	if date == "" {
		return time.Time{}, validation.Errors{"date": msgDateRequired}
	}

	parsed, ok := parseLessonDate(date, bounds)
	if !ok {
		return time.Time{}, validation.Errors{"date": msgDateInvalid}
	}

	if parsed.Before(bounds.Start) || parsed.After(bounds.End) {
		return time.Time{}, validation.Errors{"date": fmt.Sprintf("Дата вне учебного года %d/%d", year, year+1)}
	}

	existing, err := s.lessons.Find(ctx, classID, subjectID, parsed)
	if err == nil {
		allowed, err := s.canAccess(ctx, teacherID, existing)
		if err != nil {
			return time.Time{}, err
		}

		if allowed {
			return time.Time{}, validation.Errors{"date": fmt.Sprintf(msgLessonExists, parsed.Format("02.01.2006"))}
		}

		return time.Time{}, validation.Errors{"date": msgLessonTaken}
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return time.Time{}, err
	}

	allowed, err := s.canCreate(ctx, teacherID, classID, subjectID, parsed)
	if err != nil {
		return time.Time{}, err
	}

	if !allowed {
		return time.Time{}, validation.Errors{"date": msgSubstitutionInactive}
	}

	return parsed, nil
}

// parseLessonDate принимает полную дату или «ДД.ММ» без года (D-119): в
// журнале пары даты уроков показаны без года, а день и месяц однозначно
// задают дату внутри учебного года — с начала года до 31.12 это год
// начала, дальше — следующий.
func parseLessonDate(raw string, bounds YearBounds) (time.Time, bool) {
	if parsed, ok := validation.ParseDate(raw); ok {
		return parsed, true
	}

	dayMonth, err := time.ParseInLocation("2.1", raw, time.UTC)
	if err != nil {
		return time.Time{}, false
	}

	date := time.Date(bounds.Start.Year(), dayMonth.Month(), dayMonth.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(bounds.Start) {
		date = date.AddDate(1, 0, 0)
	}

	// 29.02 в невисокосный год time.Date переносит на 1 марта
	if date.Day() != dayMonth.Day() {
		return time.Time{}, false
	}

	return date, true
}
