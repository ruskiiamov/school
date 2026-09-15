package journal

import (
	"context"
	"errors"
	"log/slog"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgStudentNotInClass = "Ученик не в классе"
	msgWorkTypeRequired  = "Выберите тип работы"
	msgWorkTypeUnknown   = "Такого типа работы нет"
	msgValueRange        = "Оценка от 1 до 5"
	msgLabelTooLong      = "Подпись не длиннее 100 символов"

	maxLabelLength = 100
)

type Mark struct {
	ID           int64
	StudentID    int64
	WorkTypeID   int64
	WorkTypeName string
	Value        int
	Label        string
}

type MarkInput struct {
	WorkTypeID int64
	Value      int
	Label      string
}

type WorkType struct {
	ID   int64
	Name string
}

type StudentEntry struct {
	ID      int64
	InClass bool
	Marks   []Mark
}

func (s *Service) LessonStudents(ctx context.Context, lesson Lesson) ([]StudentEntry, error) {
	members, err := s.classStudents.StudentIDs(ctx, lesson.ClassID)
	if err != nil {
		return nil, err
	}

	marks, err := s.Marks(ctx, lesson.ID)
	if err != nil {
		return nil, err
	}

	index := make(map[int64]int, len(members))
	entries := make([]StudentEntry, 0, len(members))

	for _, id := range members {
		index[id] = len(entries)
		entries = append(entries, StudentEntry{ID: id, InClass: true})
	}

	for _, mark := range marks {
		position, ok := index[mark.StudentID]
		if !ok {
			position = len(entries)
			index[mark.StudentID] = position
			entries = append(entries, StudentEntry{ID: mark.StudentID})
		}

		entries[position].Marks = append(entries[position].Marks, mark)
	}

	return entries, nil
}

func (s *Service) Marks(ctx context.Context, lessonID int64) ([]Mark, error) {
	stored, err := s.marks.ListByLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	workTypes, err := s.workTypes.List(ctx, true)
	if err != nil {
		return nil, err
	}

	names := make(map[int64]string, len(workTypes))
	for _, workType := range workTypes {
		names[workType.ID] = workType.Name
	}

	marks := make([]Mark, 0, len(stored))
	for _, mark := range stored {
		marks = append(marks, Mark{
			ID:           mark.ID,
			StudentID:    mark.StudentID,
			WorkTypeID:   mark.WorkTypeID,
			WorkTypeName: names[mark.WorkTypeID],
			Value:        mark.Value,
			Label:        mark.Label,
		})
	}

	return marks, nil
}

func (s *Service) ActiveWorkTypes(ctx context.Context) ([]WorkType, error) {
	stored, err := s.workTypes.List(ctx, false)
	if err != nil {
		return nil, err
	}

	workTypes := make([]WorkType, 0, len(stored))
	for _, workType := range stored {
		workTypes = append(workTypes, WorkType{ID: workType.ID, Name: workType.Name})
	}

	return workTypes, nil
}

func (s *Service) AddMark(ctx context.Context, teacherID, lessonID, studentID int64, input MarkInput) (int64, error) {
	lesson, err := s.LessonForTeacher(ctx, teacherID, lessonID)
	if err != nil {
		return 0, err
	}

	member, err := s.classStudents.IsMember(ctx, lesson.ClassID, studentID)
	if err != nil {
		return 0, err
	}

	if !member {
		return 0, validation.Errors{"student": msgStudentNotInClass}
	}

	input, err = s.validateMark(ctx, input)
	if err != nil {
		return 0, err
	}

	id, err := s.marks.Create(ctx, storage.Mark{
		LessonID:   lessonID,
		StudentID:  studentID,
		WorkTypeID: input.WorkTypeID,
		Value:      input.Value,
		Label:      input.Label,
	})
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "mark added", slog.Int64("id", id), slog.Int64("lesson_id", lessonID),
		slog.Int64("student_id", studentID), slog.Int("value", input.Value))

	return id, nil
}

func (s *Service) UpdateMark(ctx context.Context, teacherID, lessonID, markID int64, input MarkInput) error {
	if _, err := s.lessonMark(ctx, teacherID, lessonID, markID); err != nil {
		return err
	}

	input, err := s.validateMark(ctx, input)
	if err != nil {
		return err
	}

	if err := s.marks.Update(ctx, markID, input.WorkTypeID, input.Value, input.Label); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "mark updated", slog.Int64("id", markID), slog.Int64("lesson_id", lessonID), slog.Int("value", input.Value))

	return nil
}

func (s *Service) DeleteMark(ctx context.Context, teacherID, lessonID, markID int64) error {
	if _, err := s.lessonMark(ctx, teacherID, lessonID, markID); err != nil {
		return err
	}

	if err := s.marks.Delete(ctx, markID); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "mark deleted", slog.Int64("id", markID), slog.Int64("lesson_id", lessonID))

	return nil
}

func (s *Service) lessonMark(ctx context.Context, teacherID, lessonID, markID int64) (storage.Mark, error) {
	if _, err := s.LessonForTeacher(ctx, teacherID, lessonID); err != nil {
		return storage.Mark{}, err
	}

	mark, err := s.marks.ByID(ctx, markID)
	if errors.Is(err, storage.ErrNotFound) || (err == nil && mark.LessonID != lessonID) {
		return storage.Mark{}, ErrNotFound
	}
	if err != nil {
		return storage.Mark{}, err
	}

	return mark, nil
}

func (s *Service) validateMark(ctx context.Context, input MarkInput) (MarkInput, error) {
	errs := validation.Errors{}

	if input.WorkTypeID == 0 {
		errs.Add("work_type", msgWorkTypeRequired)
	} else {
		workType, err := s.workTypes.ByID(ctx, input.WorkTypeID)
		if errors.Is(err, storage.ErrNotFound) || (err == nil && !workType.Active) {
			errs.Add("work_type", msgWorkTypeUnknown)
		} else if err != nil {
			return input, err
		}
	}

	if input.Value < 1 || input.Value > 5 {
		errs.Add("value", msgValueRange)
	}

	input.Label = validation.NormalizeSpaces(input.Label)
	if utf8.RuneCountInString(input.Label) > maxLabelLength {
		errs.Add("label", msgLabelTooLong)
	}

	if len(errs) > 0 {
		return input, errs
	}

	return input, nil
}
