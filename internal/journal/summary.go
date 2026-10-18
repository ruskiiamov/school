package journal

import (
	"context"
	"sort"
	"time"
)

type StudentSummary struct {
	Subjects []SubjectSummary
}

type SubjectSummary struct {
	SubjectID   int64
	SubjectName string
	Marks       []SummaryMark
	Absences    int
	Average     float64
}

type SummaryMark struct {
	LessonID     int64
	Date         time.Time
	Value        int
	WorkTypeName string
}

func (s *Service) StudentSummary(ctx context.Context, studentID int64, from, to time.Time) (StudentSummary, error) {
	marks, err := s.marks.ListByStudentPeriod(ctx, studentID, from, to)
	if err != nil {
		return StudentSummary{}, err
	}

	absences, err := s.lessonStudents.AbsencesByStudentPeriod(ctx, studentID, from, to)
	if err != nil {
		return StudentSummary{}, err
	}

	subjects, err := s.subjects.List(ctx, true)
	if err != nil {
		return StudentSummary{}, err
	}

	subjectNames := make(map[int64]string, len(subjects))
	for _, subject := range subjects {
		subjectNames[subject.ID] = subject.Name
	}

	workTypes, err := s.workTypes.List(ctx, true)
	if err != nil {
		return StudentSummary{}, err
	}

	workTypeNames := make(map[int64]string, len(workTypes))
	for _, workType := range workTypes {
		workTypeNames[workType.ID] = workType.Name
	}

	index := map[int64]int{}
	rows := []SubjectSummary{}

	row := func(subjectID int64) *SubjectSummary {
		if at, ok := index[subjectID]; ok {
			return &rows[at]
		}

		index[subjectID] = len(rows)
		rows = append(rows, SubjectSummary{SubjectID: subjectID, SubjectName: subjectNames[subjectID]})

		return &rows[len(rows)-1]
	}

	for _, mark := range marks {
		current := row(mark.SubjectID)
		current.Marks = append(current.Marks, SummaryMark{
			LessonID:     mark.LessonID,
			Date:         mark.Date,
			Value:        mark.Value,
			WorkTypeName: workTypeNames[mark.WorkTypeID],
		})
		current.Average += float64(mark.Value)
	}

	for subjectID, count := range absences {
		row(subjectID).Absences = count
	}

	for i := range rows {
		if len(rows[i].Marks) > 0 {
			rows[i].Average /= float64(len(rows[i].Marks))
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].SubjectName < rows[j].SubjectName
	})

	return StudentSummary{Subjects: rows}, nil
}
