package journal

import (
	"context"
	"sort"
	"time"
)

type DiaryLesson struct {
	ID          int64
	TeacherID   int64
	Date        time.Time
	Topic       string
	SubjectName string
	Marks       []Mark
	Absent      bool
	Comment     string
}

func (s *Service) DayLessons(ctx context.Context, studentID int64, date time.Time) ([]DiaryLesson, error) {
	stored, err := s.lessons.ListForStudent(ctx, studentID, date)
	if err != nil {
		return nil, err
	}

	if len(stored) == 0 {
		return nil, nil
	}

	subjects, err := s.subjects.List(ctx, true)
	if err != nil {
		return nil, err
	}

	subjectNames := make(map[int64]string, len(subjects))
	for _, subject := range subjects {
		subjectNames[subject.ID] = subject.Name
	}

	lessons := make([]DiaryLesson, 0, len(stored))

	for _, item := range stored {
		lesson := DiaryLesson{
			ID:          item.ID,
			TeacherID:   item.TeacherID,
			Date:        item.Date,
			Topic:       item.Topic,
			SubjectName: subjectNames[item.SubjectID],
		}

		marks, err := s.Marks(ctx, item.ID)
		if err != nil {
			return nil, err
		}

		for _, mark := range marks {
			if mark.StudentID == studentID {
				lesson.Marks = append(lesson.Marks, mark)
			}
		}

		records, err := s.lessonStudents.ListByLesson(ctx, item.ID)
		if err != nil {
			return nil, err
		}

		for _, record := range records {
			if record.StudentID == studentID {
				lesson.Absent = record.Absent
				lesson.Comment = record.Comment
			}
		}

		lessons = append(lessons, lesson)
	}

	sort.SliceStable(lessons, func(i, j int) bool {
		return lessons[i].SubjectName < lessons[j].SubjectName
	})

	return lessons, nil
}
