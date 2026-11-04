package journal

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

type MarkKey struct {
	WorkTypeID int64
	StudentID  int64
}

func (k MarkKey) Field() string {
	return "m-" + strconv.FormatInt(k.WorkTypeID, 10) + "-" + strconv.FormatInt(k.StudentID, 10)
}

type LessonMark struct {
	Value   string
	Comment string
}

type LessonMarks struct {
	Lesson     Lesson
	Types      []WorkType
	Members    map[int64]bool
	Absent     map[int64]bool
	Marks      map[MarkKey]LessonMark
	CanAddType bool
}

func (m LessonMarks) Editable(key MarkKey) bool {
	if _, marked := m.Marks[key]; marked {
		return true
	}

	if !m.Members[key.StudentID] {
		return false
	}

	for _, workType := range m.Types {
		if workType.ID == key.WorkTypeID {
			return workType.Active
		}
	}

	return false
}

func (s *Service) LessonMarks(ctx context.Context, teacherID, lessonID int64) (LessonMarks, error) {
	lesson, err := s.LessonForTeacher(ctx, teacherID, lessonID)
	if err != nil {
		return LessonMarks{}, err
	}

	members, err := s.classStudents.StudentIDs(ctx, lesson.ClassID)
	if err != nil {
		return LessonMarks{}, err
	}

	stored, err := s.marks.ListByLesson(ctx, lessonID)
	if err != nil {
		return LessonMarks{}, err
	}

	records, err := s.lessonStudents.ListByLesson(ctx, lessonID)
	if err != nil {
		return LessonMarks{}, err
	}

	workTypes, err := s.WorkTypes(ctx, lesson.TeacherID, lesson.SubjectID, true)
	if err != nil {
		return LessonMarks{}, err
	}

	result := LessonMarks{
		Lesson:     lesson,
		Members:    make(map[int64]bool, len(members)),
		Absent:     map[int64]bool{},
		Marks:      make(map[MarkKey]LessonMark, len(stored)),
		CanAddType: lesson.TeacherID == teacherID,
	}

	for _, id := range members {
		result.Members[id] = true
	}

	used := map[int64]bool{}

	for _, mark := range stored {
		result.Marks[MarkKey{WorkTypeID: mark.WorkTypeID, StudentID: mark.StudentID}] = LessonMark{Value: mark.Value, Comment: mark.Comment}
		used[mark.WorkTypeID] = true
	}

	for _, record := range records {
		if record.Absent {
			result.Absent[record.StudentID] = true
		}
	}

	for _, workType := range workTypes {
		if workType.Active || used[workType.ID] {
			result.Types = append(result.Types, workType)
		}
	}

	return result, nil
}

func (s *Service) SaveLessonMarks(ctx context.Context, teacherID, lessonID int64, values map[MarkKey]string) error {
	marks, err := s.LessonMarks(ctx, teacherID, lessonID)
	if err != nil {
		return err
	}

	errs := validation.Errors{}
	changes := []storage.MarkChange{}

	for key, raw := range values {
		if !marks.Editable(key) {
			return ErrForbidden
		}

		value, ok := parseMarkValue(raw)
		if !ok {
			errs.Add(key.Field(), msgValueRange)
			continue
		}

		current := marks.Marks[key]
		if value == current.Value {
			continue
		}

		change := storage.MarkChange{StudentID: key.StudentID, WorkTypeID: key.WorkTypeID, Value: value}
		if value != "" {
			change.Comment = current.Comment
		}

		changes = append(changes, change)
	}

	if len(errs) > 0 {
		return errs
	}

	if len(changes) == 0 {
		return nil
	}

	if err := s.marks.Set(ctx, lessonID, changes); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "lesson marks saved", slog.Int64("lesson_id", lessonID), slog.Int("changes", len(changes)),
		slog.Int64("teacher_id", teacherID))

	return nil
}

func (s *Service) CreateLessonWorkType(ctx context.Context, teacherID, lessonID int64, name string) (int64, error) {
	lesson, err := s.LessonForTeacher(ctx, teacherID, lessonID)
	if err != nil {
		return 0, err
	}

	if lesson.TeacherID != teacherID {
		return 0, ErrForbidden
	}

	return s.CreateWorkType(ctx, teacherID, lesson.SubjectID, name)
}
