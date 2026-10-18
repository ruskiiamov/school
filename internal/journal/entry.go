package journal

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

type EntryType struct {
	ID    int64
	Name  string
	Value int
}

type MarksEntry struct {
	Lesson    Lesson
	StudentID int64
	InClass   bool
	Types     []EntryType
	Absent    bool
	Comment   string
}

type MarksEntryInput struct {
	Values  map[int64]string
	Absent  bool
	Comment string
}

func EntryTypeKey(workTypeID int64) string {
	return "type-" + strconv.FormatInt(workTypeID, 10)
}

func (s *Service) MarksEntry(ctx context.Context, teacherID, lessonID, studentID int64) (MarksEntry, error) {
	lesson, err := s.LessonForTeacher(ctx, teacherID, lessonID)
	if err != nil {
		return MarksEntry{}, err
	}

	return s.marksEntry(ctx, lesson, studentID)
}

func (s *Service) AdminMarksEntry(ctx context.Context, lessonID, studentID int64) (MarksEntry, error) {
	lesson, err := s.LessonByID(ctx, lessonID)
	if err != nil {
		return MarksEntry{}, err
	}

	return s.marksEntry(ctx, lesson, studentID)
}

func (s *Service) marksEntry(ctx context.Context, lesson Lesson, studentID int64) (MarksEntry, error) {
	lessonID := lesson.ID

	inClass, err := s.classStudents.IsMember(ctx, lesson.ClassID, studentID)
	if err != nil {
		return MarksEntry{}, err
	}

	if !inClass {
		known, err := s.knownInPair(ctx, lesson, studentID)
		if err != nil {
			return MarksEntry{}, err
		}

		if !known {
			return MarksEntry{}, ErrNotFound
		}
	}

	entry := MarksEntry{Lesson: lesson, StudentID: studentID, InClass: inClass}

	stored, err := s.marks.ListByLesson(ctx, lessonID)
	if err != nil {
		return MarksEntry{}, err
	}

	values := map[int64]int{}
	for _, mark := range stored {
		if mark.StudentID == studentID {
			values[mark.WorkTypeID] = mark.Value
		}
	}

	workTypes, err := s.workTypes.List(ctx, true)
	if err != nil {
		return MarksEntry{}, err
	}

	for _, workType := range workTypes {
		value, marked := values[workType.ID]
		if marked || (inClass && workType.Active) {
			entry.Types = append(entry.Types, EntryType{ID: workType.ID, Name: workType.Name, Value: value})
		}
	}

	records, err := s.lessonStudents.ListByLesson(ctx, lessonID)
	if err != nil {
		return MarksEntry{}, err
	}

	for _, record := range records {
		if record.StudentID == studentID {
			entry.Absent = record.Absent
			entry.Comment = record.Comment
		}
	}

	return entry, nil
}

func (s *Service) knownInPair(ctx context.Context, lesson Lesson, studentID int64) (bool, error) {
	marks, err := s.marks.ListByPair(ctx, lesson.ClassID, lesson.SubjectID)
	if err != nil {
		return false, err
	}

	for _, mark := range marks {
		if mark.StudentID == studentID {
			return true, nil
		}
	}

	records, err := s.lessonStudents.ListByPair(ctx, lesson.ClassID, lesson.SubjectID)
	if err != nil {
		return false, err
	}

	for _, record := range records {
		if record.StudentID == studentID {
			return true, nil
		}
	}

	return false, nil
}

func (s *Service) SaveMarksEntry(ctx context.Context, teacherID, lessonID, studentID int64, input MarksEntryInput) error {
	entry, err := s.MarksEntry(ctx, teacherID, lessonID, studentID)
	if err != nil {
		return err
	}

	errs := validation.Errors{}
	changes := []storage.MarkChange{}

	for _, workType := range entry.Types {
		value, ok := parseMarkValue(input.Values[workType.ID])
		if !ok {
			errs.Add(EntryTypeKey(workType.ID), msgValueRange)
			continue
		}

		if value != workType.Value {
			changes = append(changes, storage.MarkChange{WorkTypeID: workType.ID, Value: value})
		}
	}

	record := RecordInput{Absent: input.Absent, Comment: input.Comment}

	if entry.InClass {
		var fieldErrs validation.Errors

		record, err = validateRecord(record)
		if errors.As(err, &fieldErrs) {
			for key, message := range fieldErrs {
				errs.Add(key, message)
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}

	if len(changes) > 0 {
		if err := s.marks.SetForStudent(ctx, lessonID, studentID, changes); err != nil {
			return err
		}

		s.log.InfoContext(ctx, "marks saved", slog.Int64("lesson_id", lessonID), slog.Int64("student_id", studentID),
			slog.Int("changes", len(changes)), slog.Int64("teacher_id", teacherID))
	}

	if entry.InClass && (record.Absent != entry.Absent || record.Comment != entry.Comment) {
		return s.storeRecord(ctx, lessonID, studentID, record)
	}

	return nil
}

func parseMarkValue(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 5 {
		return 0, false
	}

	return value, true
}
