package journal

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/files"
	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgDateRequired         = "Укажите дату"
	msgDateInvalid          = "Неверная дата"
	msgPairUnknown          = "Выберите класс и предмет из списка"
	msgLessonTaken          = "Урок на эту дату уже создал другой учитель"
	msgSubstitutionInactive = "На эту дату замена не действует"
	msgTopicTooLong         = "Тема не длиннее 200 символов"
	msgLessonNotEmpty       = "Урок с оценками или записями удалить нельзя"

	maxTopicLength = 200
	recentLimit    = 15
)

type Lesson struct {
	ID          int64
	ClassID     int64
	SubjectID   int64
	TeacherID   int64
	Date        time.Time
	Topic       string
	ClassName   string
	SubjectName string
}

type OpenLessonInput struct {
	ClassID   int64
	SubjectID int64
	Date      string
}

func (s *Service) OpenLesson(ctx context.Context, teacherID int64, year int, today time.Time, input OpenLessonInput) (int64, error) {
	errs := validation.Errors{}

	pairs, err := s.Pairs(ctx, teacherID, year, today)
	if err != nil {
		return 0, err
	}

	if !containsPair(pairs, input.ClassID, input.SubjectID) {
		errs.Add("pair", msgPairUnknown)
	}

	var date time.Time

	if input.Date == "" {
		errs.Add("date", msgDateRequired)
	} else if parsed, ok := validation.ParseDate(input.Date); ok {
		date = parsed
	} else {
		errs.Add("date", msgDateInvalid)
	}

	if len(errs) > 0 {
		return 0, errs
	}

	existing, err := s.lessons.Find(ctx, input.ClassID, input.SubjectID, date)
	if err == nil {
		allowed, err := s.canAccess(ctx, teacherID, existing)
		if err != nil {
			return 0, err
		}

		if !allowed {
			return 0, validation.Errors{"pair": msgLessonTaken}
		}

		return existing.ID, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return 0, err
	}

	allowed, err := s.canCreate(ctx, teacherID, input.ClassID, input.SubjectID, date)
	if err != nil {
		return 0, err
	}

	if !allowed {
		return 0, validation.Errors{"date": msgSubstitutionInactive}
	}

	id, err := s.lessons.Create(ctx, storage.Lesson{
		ClassID:   input.ClassID,
		SubjectID: input.SubjectID,
		TeacherID: teacherID,
		Date:      date,
	})
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "lesson created", slog.Int64("id", id), slog.Int64("teacher_id", teacherID),
		slog.Int64("class_id", input.ClassID), slog.Int64("subject_id", input.SubjectID), slog.String("date", input.Date))

	return id, nil
}

func (s *Service) LessonForTeacher(ctx context.Context, teacherID, id int64) (Lesson, error) {
	stored, err := s.lessons.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return Lesson{}, ErrNotFound
	}
	if err != nil {
		return Lesson{}, err
	}

	allowed, err := s.canAccess(ctx, teacherID, stored)
	if err != nil {
		return Lesson{}, err
	}

	if !allowed {
		return Lesson{}, ErrForbidden
	}

	return s.withNames(ctx, stored)
}

func (s *Service) LessonByID(ctx context.Context, id int64) (Lesson, error) {
	stored, err := s.lessons.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return Lesson{}, ErrNotFound
	}
	if err != nil {
		return Lesson{}, err
	}

	return s.withNames(ctx, stored)
}

func (s *Service) withNames(ctx context.Context, stored storage.Lesson) (Lesson, error) {
	lesson := toLesson(stored)

	class, err := s.classes.ByID(ctx, stored.ClassID)
	if err != nil {
		return Lesson{}, err
	}

	subject, err := s.subjects.ByID(ctx, stored.SubjectID)
	if err != nil {
		return Lesson{}, err
	}

	lesson.ClassName = class.Name
	lesson.SubjectName = subject.Name

	return lesson, nil
}

func (s *Service) LessonsByPair(ctx context.Context, classID, subjectID int64) ([]Lesson, map[int64]bool, error) {
	stored, err := s.lessons.ListByPair(ctx, classID, subjectID)
	if err != nil {
		return nil, nil, err
	}

	ids, err := s.homework.LessonIDsByPair(ctx, classID, subjectID)
	if err != nil {
		return nil, nil, err
	}

	withHomework := make(map[int64]bool, len(ids))
	for _, id := range ids {
		withHomework[id] = true
	}

	lessons := make([]Lesson, 0, len(stored))
	for _, item := range stored {
		lessons = append(lessons, toLesson(item))
	}

	return lessons, withHomework, nil
}

func (s *Service) RecentLessons(ctx context.Context, teacherID int64, year int) ([]Lesson, error) {
	stored, err := s.lessons.ListVisible(ctx, teacherID, year, recentLimit)
	if err != nil {
		return nil, err
	}

	classes, err := s.classes.ListByYear(ctx, year, true)
	if err != nil {
		return nil, err
	}

	classNames := make(map[int64]string, len(classes))
	for _, class := range classes {
		classNames[class.ID] = class.Name
	}

	subjects, err := s.subjects.List(ctx, true)
	if err != nil {
		return nil, err
	}

	subjectNames := make(map[int64]string, len(subjects))
	for _, subject := range subjects {
		subjectNames[subject.ID] = subject.Name
	}

	lessons := make([]Lesson, 0, len(stored))
	for _, item := range stored {
		lesson := toLesson(item)
		lesson.ClassName = classNames[item.ClassID]
		lesson.SubjectName = subjectNames[item.SubjectID]
		lessons = append(lessons, lesson)
	}

	return lessons, nil
}

func (s *Service) UpdateTopic(ctx context.Context, teacherID, id int64, topic string) error {
	if _, err := s.LessonForTeacher(ctx, teacherID, id); err != nil {
		return err
	}

	topic = validation.NormalizeSpaces(topic)
	if utf8.RuneCountInString(topic) > maxTopicLength {
		return validation.Errors{"topic": msgTopicTooLong}
	}

	if err := s.lessons.UpdateTopic(ctx, id, topic); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "lesson topic updated", slog.Int64("id", id), slog.Int64("teacher_id", teacherID))

	return nil
}

func (s *Service) CanDeleteLesson(ctx context.Context, id int64) (bool, error) {
	has, err := s.lessons.HasRecords(ctx, id)
	if err != nil {
		return false, err
	}

	return !has, nil
}

func (s *Service) DeleteLesson(ctx context.Context, teacherID, id int64) error {
	if _, err := s.LessonForTeacher(ctx, teacherID, id); err != nil {
		return err
	}

	empty, err := s.CanDeleteLesson(ctx, id)
	if err != nil {
		return err
	}

	if !empty {
		return validation.Errors{"lesson": msgLessonNotEmpty}
	}

	homeworkFiles, err := s.homeworkFiles.ListByLesson(ctx, id)
	if err != nil {
		return err
	}

	if err := s.lessons.DeleteEmpty(ctx, id); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	for _, file := range homeworkFiles {
		if err := s.store.Delete(file.ID); err != nil && !errors.Is(err, files.ErrNotFound) {
			s.log.ErrorContext(ctx, "remove homework file", slog.String("file_id", file.ID), slog.Any("error", err))
		}
	}

	s.log.InfoContext(ctx, "lesson deleted", slog.Int64("id", id), slog.Int64("teacher_id", teacherID),
		slog.Int("homework_files", len(homeworkFiles)))

	return nil
}

func (s *Service) canAccess(ctx context.Context, teacherID int64, lesson storage.Lesson) (bool, error) {
	if lesson.TeacherID == teacherID {
		return true, nil
	}

	return s.assignments.IsAssigned(ctx, lesson.ClassID, lesson.SubjectID, teacherID)
}

func (s *Service) canCreate(ctx context.Context, teacherID, classID, subjectID int64, date time.Time) (bool, error) {
	assigned, err := s.assignments.IsAssigned(ctx, classID, subjectID, teacherID)
	if err != nil || assigned {
		return assigned, err
	}

	return s.substitutions.CoversDate(ctx, classID, subjectID, teacherID, date)
}

func containsPair(pairs []Pair, classID, subjectID int64) bool {
	for _, pair := range pairs {
		if pair.ClassID == classID && pair.SubjectID == subjectID {
			return true
		}
	}

	return false
}

func toLesson(stored storage.Lesson) Lesson {
	return Lesson{
		ID:        stored.ID,
		ClassID:   stored.ClassID,
		SubjectID: stored.SubjectID,
		TeacherID: stored.TeacherID,
		Date:      stored.Date,
		Topic:     stored.Topic,
	}
}
