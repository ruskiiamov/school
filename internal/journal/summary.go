package journal

import (
	"context"
	"sort"
	"time"

	"github.com/ruskiiamov/school/internal/storage"
)

type Grid struct {
	Lessons []Lesson
	Rows    []GridRow
}

type GridRow struct {
	StudentID int64
	InClass   bool
	Cells     map[int64]GridCell
	Absences  int
	Average   float64
	MarkCount int
}

type GridCell struct {
	Marks  []int
	Absent bool
}

func (s *Service) TeacherGrid(ctx context.Context, teacherID, classID, subjectID int64, from, to time.Time) (Grid, error) {
	assigned, err := s.assignments.IsAssigned(ctx, classID, subjectID, teacherID)
	if err != nil {
		return Grid{}, err
	}

	lessons, err := s.lessons.ListByPairPeriod(ctx, classID, subjectID, from, to)
	if err != nil {
		return Grid{}, err
	}

	visible := make([]storage.Lesson, 0, len(lessons))

	for _, lesson := range lessons {
		if assigned || lesson.TeacherID == teacherID {
			visible = append(visible, lesson)
		}
	}

	return s.grid(ctx, classID, subjectID, from, to, visible)
}

func (s *Service) PairGrid(ctx context.Context, classID, subjectID int64, from, to time.Time) (Grid, error) {
	lessons, err := s.lessons.ListByPairPeriod(ctx, classID, subjectID, from, to)
	if err != nil {
		return Grid{}, err
	}

	return s.grid(ctx, classID, subjectID, from, to, lessons)
}

func (s *Service) grid(ctx context.Context, classID, subjectID int64, from, to time.Time, lessons []storage.Lesson) (Grid, error) {
	grid := Grid{Lessons: make([]Lesson, 0, len(lessons))}
	included := make(map[int64]bool, len(lessons))

	for _, lesson := range lessons {
		grid.Lessons = append(grid.Lessons, toLesson(lesson))
		included[lesson.ID] = true
	}

	members, err := s.classStudents.StudentIDs(ctx, classID)
	if err != nil {
		return Grid{}, err
	}

	marks, err := s.marks.ListByPairPeriod(ctx, classID, subjectID, from, to)
	if err != nil {
		return Grid{}, err
	}

	records, err := s.lessonStudents.ListByPairPeriod(ctx, classID, subjectID, from, to)
	if err != nil {
		return Grid{}, err
	}

	index := map[int64]int{}
	rows := []GridRow{}

	row := func(studentID int64) *GridRow {
		if at, ok := index[studentID]; ok {
			return &rows[at]
		}

		index[studentID] = len(rows)
		rows = append(rows, GridRow{StudentID: studentID, Cells: map[int64]GridCell{}})

		return &rows[len(rows)-1]
	}

	for _, id := range members {
		row(id).InClass = true
	}

	for _, mark := range marks {
		if !included[mark.LessonID] {
			continue
		}

		current := row(mark.StudentID)
		cell := current.Cells[mark.LessonID]
		cell.Marks = append(cell.Marks, mark.Value)
		current.Cells[mark.LessonID] = cell
		current.MarkCount++
		current.Average += float64(mark.Value)
	}

	for _, record := range records {
		if !included[record.LessonID] || !record.Absent {
			continue
		}

		current := row(record.StudentID)
		cell := current.Cells[record.LessonID]
		cell.Absent = true
		current.Cells[record.LessonID] = cell
		current.Absences++
	}

	for i := range rows {
		if rows[i].MarkCount > 0 {
			rows[i].Average /= float64(rows[i].MarkCount)
		}
	}

	grid.Rows = rows

	return grid, nil
}

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
	Label        string
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
			Label:        mark.Label,
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
