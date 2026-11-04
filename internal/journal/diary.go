package journal

import (
	"context"
	"sort"
	"time"
)

type LessonPenalty struct {
	WorkTypeID   int64
	WorkTypeName string
	Penalty
}

type DiaryLesson struct {
	ID          int64
	TeacherID   int64
	Date        time.Time
	Topic       string
	SubjectName string
	Marks       []Mark
	Penalties   []LessonPenalty
	Absent      bool
	Comment     string
	Homework    Homework
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

	ledger, penalties, err := s.ledger(ctx, []int64{studentID})
	if err != nil {
		return nil, err
	}

	typeIDs := make([]int64, 0, len(penalties))
	for _, penalty := range penalties {
		typeIDs = append(typeIDs, penalty.WorkTypeID)
	}

	typeNames, typeOrder, err := s.workTypeNames(ctx, typeIDs)
	if err != nil {
		return nil, err
	}

	lessonPenalties := map[int64][]LessonPenalty{}
	for _, penalty := range penalties {
		lessonPenalties[penalty.LessonID] = append(lessonPenalties[penalty.LessonID], LessonPenalty{
			WorkTypeID:   penalty.WorkTypeID,
			WorkTypeName: typeNames[penalty.WorkTypeID],
			Penalty:      Penalty{DonePercent: penalty.DonePercent, Points: penalty.Points, Status: ledger.statuses[penalty.ID]},
		})
	}

	for _, list := range lessonPenalties {
		sort.SliceStable(list, func(i, j int) bool {
			return typeOrder[list[i].WorkTypeID] < typeOrder[list[j].WorkTypeID]
		})
	}

	lessons := make([]DiaryLesson, 0, len(stored))

	for _, item := range stored {
		lesson := DiaryLesson{
			ID:          item.ID,
			TeacherID:   item.TeacherID,
			Date:        item.Date,
			Topic:       item.Topic,
			SubjectName: subjectNames[item.SubjectID],
			Penalties:   lessonPenalties[item.ID],
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

		if lesson.Homework, err = s.Homework(ctx, item.ID); err != nil {
			return nil, err
		}

		lessons = append(lessons, lesson)
	}

	sort.SliceStable(lessons, func(i, j int) bool {
		return lessons[i].SubjectName < lessons[j].SubjectName
	})

	return lessons, nil
}
