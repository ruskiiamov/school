package journal

import (
	"context"
	"errors"
	"sort"

	"github.com/ruskiiamov/school/internal/storage"
)

type JournalMark struct {
	WorkTypeName string
	Value        int
}

type JournalCell struct {
	Marks   []JournalMark
	Absent  bool
	Comment string
}

type CellKey struct {
	LessonID  int64
	StudentID int64
}

type JournalStudent struct {
	ID        int64
	InClass   bool
	Absences  int
	Average   float64
	MarkCount int
}

type PairJournal struct {
	Pair         YearPair
	TeacherID    int64
	Year         int
	Lessons      []Lesson
	WithHomework map[int64]bool
	Students     []JournalStudent
	Cells        map[CellKey]JournalCell
}

func (s *Service) PairJournal(ctx context.Context, teacherID, classID, subjectID int64) (PairJournal, error) {
	class, err := s.classes.ByID(ctx, classID)
	if errors.Is(err, storage.ErrNotFound) {
		return PairJournal{}, ErrNotFound
	}
	if err != nil {
		return PairJournal{}, err
	}

	pair, err := s.yearPair(ctx, teacherID, class.Year, classID, subjectID)
	if err != nil {
		return PairJournal{}, err
	}

	stored, err := s.lessons.ListByPair(ctx, classID, subjectID)
	if err != nil {
		return PairJournal{}, err
	}

	journal := PairJournal{Pair: pair, Year: class.Year, Cells: map[CellKey]JournalCell{}}
	included := map[int64]bool{}

	for _, item := range stored {
		lesson := toLesson(item)
		if pair.Kind != PairAssigned && (lesson.TeacherID != teacherID || !pair.Shows(lesson)) {
			continue
		}

		journal.Lessons = append(journal.Lessons, lesson)
		included[lesson.ID] = true
	}

	if journal.WithHomework, err = s.homeworkLessons(ctx, classID, subjectID); err != nil {
		return PairJournal{}, err
	}

	if err := s.fillJournalCells(ctx, &journal, included); err != nil {
		return PairJournal{}, err
	}

	return journal, nil
}

func (s *Service) AdminPairJournal(ctx context.Context, classID, subjectID int64) (PairJournal, error) {
	class, err := s.classes.ByID(ctx, classID)
	if errors.Is(err, storage.ErrNotFound) {
		return PairJournal{}, ErrNotFound
	}
	if err != nil {
		return PairJournal{}, err
	}

	subject, err := s.subjects.ByID(ctx, subjectID)
	if errors.Is(err, storage.ErrNotFound) {
		return PairJournal{}, ErrNotFound
	}
	if err != nil {
		return PairJournal{}, err
	}

	assignments, err := s.assignments.ListByClass(ctx, classID)
	if err != nil {
		return PairJournal{}, err
	}

	journal := PairJournal{
		Pair: YearPair{
			Pair: Pair{ClassID: classID, SubjectID: subjectID, ClassName: class.Name, SubjectName: subject.Name},
			Kind: PairAssigned,
		},
		Year:  class.Year,
		Cells: map[CellKey]JournalCell{},
	}

	for _, assignment := range assignments {
		if assignment.SubjectID == subjectID {
			journal.TeacherID = assignment.TeacherID
		}
	}

	stored, err := s.lessons.ListByPair(ctx, classID, subjectID)
	if err != nil {
		return PairJournal{}, err
	}

	included := map[int64]bool{}

	for _, item := range stored {
		journal.Lessons = append(journal.Lessons, toLesson(item))
		included[item.ID] = true
	}

	if journal.WithHomework, err = s.homeworkLessons(ctx, classID, subjectID); err != nil {
		return PairJournal{}, err
	}

	if err := s.fillJournalCells(ctx, &journal, included); err != nil {
		return PairJournal{}, err
	}

	return journal, nil
}

func (s *Service) yearPair(ctx context.Context, teacherID int64, year int, classID, subjectID int64) (YearPair, error) {
	pairs, err := s.YearPairs(ctx, teacherID, year)
	if err != nil {
		return YearPair{}, err
	}

	for _, pair := range pairs {
		if pair.ClassID == classID && pair.SubjectID == subjectID {
			return pair, nil
		}
	}

	return YearPair{}, ErrForbidden
}

func (s *Service) homeworkLessons(ctx context.Context, classID, subjectID int64) (map[int64]bool, error) {
	ids, err := s.homework.LessonIDsByPair(ctx, classID, subjectID)
	if err != nil {
		return nil, err
	}

	withHomework := make(map[int64]bool, len(ids))
	for _, id := range ids {
		withHomework[id] = true
	}

	return withHomework, nil
}

func (s *Service) fillJournalCells(ctx context.Context, journal *PairJournal, included map[int64]bool) error {
	classID, subjectID := journal.Pair.ClassID, journal.Pair.SubjectID

	members, err := s.classStudents.StudentIDs(ctx, classID)
	if err != nil {
		return err
	}

	marks, err := s.marks.ListByPair(ctx, classID, subjectID)
	if err != nil {
		return err
	}

	records, err := s.lessonStudents.ListByPair(ctx, classID, subjectID)
	if err != nil {
		return err
	}

	workTypes, err := s.workTypes.List(ctx, true)
	if err != nil {
		return err
	}

	names := make(map[int64]string, len(workTypes))
	order := make(map[int64]int, len(workTypes))

	for i, workType := range workTypes {
		names[workType.ID] = workType.Name
		order[workType.ID] = i
	}

	index := map[int64]int{}

	student := func(id int64) *JournalStudent {
		if at, ok := index[id]; ok {
			return &journal.Students[at]
		}

		index[id] = len(journal.Students)
		journal.Students = append(journal.Students, JournalStudent{ID: id})

		return &journal.Students[len(journal.Students)-1]
	}

	for _, id := range members {
		student(id).InClass = true
	}

	sort.SliceStable(marks, func(i, j int) bool {
		return order[marks[i].WorkTypeID] < order[marks[j].WorkTypeID]
	})

	for _, mark := range marks {
		if !included[mark.LessonID] {
			continue
		}

		current := student(mark.StudentID)
		current.MarkCount++
		current.Average += float64(mark.Value)

		key := CellKey{LessonID: mark.LessonID, StudentID: mark.StudentID}
		cell := journal.Cells[key]
		cell.Marks = append(cell.Marks, JournalMark{WorkTypeName: names[mark.WorkTypeID], Value: mark.Value})
		journal.Cells[key] = cell
	}

	for _, record := range records {
		if !included[record.LessonID] {
			continue
		}

		current := student(record.StudentID)
		if record.Absent {
			current.Absences++
		}

		key := CellKey{LessonID: record.LessonID, StudentID: record.StudentID}
		cell := journal.Cells[key]
		cell.Absent = record.Absent
		cell.Comment = record.Comment
		journal.Cells[key] = cell
	}

	for i := range journal.Students {
		if journal.Students[i].MarkCount > 0 {
			journal.Students[i].Average /= float64(journal.Students[i].MarkCount)
		}
	}

	return nil
}
