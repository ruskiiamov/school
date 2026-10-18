package journal

import (
	"context"
	"sort"
	"time"

	"github.com/ruskiiamov/school/internal/storage"
)

type Pair struct {
	ClassID     int64
	SubjectID   int64
	ClassName   string
	SubjectName string
}

type PairKind int

const (
	PairAssigned PairKind = iota
	PairSubstitution
	PairOwnLessons
)

type SubstitutionPeriod struct {
	Start time.Time
	End   time.Time
}

func (p SubstitutionPeriod) Covers(date time.Time) bool {
	return !date.Before(p.Start) && (p.End.IsZero() || !date.After(p.End))
}

type YearPair struct {
	Pair
	Kind        PairKind
	Periods     []SubstitutionPeriod
	LessonCount int
}

func (p YearPair) Shows(lesson Lesson) bool {
	if p.Kind != PairSubstitution {
		return true
	}

	for _, period := range p.Periods {
		if period.Covers(lesson.Date) {
			return true
		}
	}

	return false
}

func (s *Service) YearPairs(ctx context.Context, teacherID int64, year int) ([]YearPair, error) {
	classes, err := s.classes.ListByYear(ctx, year, true)
	if err != nil {
		return nil, err
	}

	subjects, err := s.subjects.List(ctx, true)
	if err != nil {
		return nil, err
	}

	assignments, err := s.assignments.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, err
	}

	substitutions, err := s.substitutions.ListByTeacherYear(ctx, teacherID, year)
	if err != nil {
		return nil, err
	}

	lessons, err := s.lessons.ListVisibleByYear(ctx, teacherID, year)
	if err != nil {
		return nil, err
	}

	classByID := make(map[int64]storage.Class, len(classes))
	order := make(map[int64]int, len(classes))

	for i, class := range classes {
		classByID[class.ID] = class
		order[class.ID] = i
	}

	subjectByID := make(map[int64]storage.Subject, len(subjects))
	for _, subject := range subjects {
		subjectByID[subject.ID] = subject
	}

	pairs := map[[2]int64]*YearPair{}

	add := func(classID, subjectID int64, kind PairKind) *YearPair {
		key := [2]int64{classID, subjectID}
		if pair, ok := pairs[key]; ok {
			return pair
		}

		pairs[key] = &YearPair{
			Pair: Pair{
				ClassID:     classID,
				SubjectID:   subjectID,
				ClassName:   classByID[classID].Name,
				SubjectName: subjectByID[subjectID].Name,
			},
			Kind: kind,
		}

		return pairs[key]
	}

	active := func(classID, subjectID int64) bool {
		class, classOK := classByID[classID]
		subject, subjectOK := subjectByID[subjectID]

		return classOK && subjectOK && class.Active && subject.Active
	}

	for _, assignment := range assignments {
		if active(assignment.ClassID, assignment.SubjectID) {
			add(assignment.ClassID, assignment.SubjectID, PairAssigned)
		}
	}

	for _, substitution := range substitutions {
		if !active(substitution.ClassID, substitution.SubjectID) {
			continue
		}

		pair := add(substitution.ClassID, substitution.SubjectID, PairSubstitution)
		if pair.Kind == PairSubstitution {
			pair.Periods = append(pair.Periods, SubstitutionPeriod{Start: substitution.StartDate, End: substitution.EndDate})
		}
	}

	for _, stored := range lessons {
		pair, ok := pairs[[2]int64{stored.ClassID, stored.SubjectID}]
		if !ok {
			if stored.TeacherID != teacherID {
				continue
			}

			pair = add(stored.ClassID, stored.SubjectID, PairOwnLessons)
		}

		if !pair.Shows(toLesson(stored)) {
			continue
		}

		pair.LessonCount++
	}

	result := make([]YearPair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, *pair)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].ClassID != result[j].ClassID {
			return order[result[i].ClassID] < order[result[j].ClassID]
		}

		return result[i].SubjectName < result[j].SubjectName
	})

	return result, nil
}
