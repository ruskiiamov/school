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

func (s *Service) Pairs(ctx context.Context, teacherID int64, year int, today time.Time) ([]Pair, error) {
	assignments, err := s.assignments.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, err
	}

	substitutions, err := s.substitutions.ListActiveByTeacher(ctx, teacherID, today)
	if err != nil {
		return nil, err
	}

	classes, err := s.activeClasses(ctx, year)
	if err != nil {
		return nil, err
	}

	subjects, err := s.activeSubjects(ctx)
	if err != nil {
		return nil, err
	}

	seen := map[[2]int64]bool{}
	pairs := []Pair{}

	add := func(classID, subjectID int64) {
		class, classOK := classes[classID]
		subject, subjectOK := subjects[subjectID]
		key := [2]int64{classID, subjectID}

		if !classOK || !subjectOK || seen[key] {
			return
		}

		seen[key] = true
		pairs = append(pairs, Pair{ClassID: classID, SubjectID: subjectID, ClassName: class.Name, SubjectName: subject.Name})
	}

	for _, assignment := range assignments {
		add(assignment.ClassID, assignment.SubjectID)
	}

	for _, substitution := range substitutions {
		add(substitution.ClassID, substitution.SubjectID)
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].ClassName != pairs[j].ClassName {
			return pairs[i].ClassName < pairs[j].ClassName
		}

		return pairs[i].SubjectName < pairs[j].SubjectName
	})

	return pairs, nil
}

func (s *Service) activeClasses(ctx context.Context, year int) (map[int64]storage.Class, error) {
	classes, err := s.classes.ListByYear(ctx, year, false)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]storage.Class, len(classes))
	for _, class := range classes {
		byID[class.ID] = class
	}

	return byID, nil
}

func (s *Service) activeSubjects(ctx context.Context) (map[int64]storage.Subject, error) {
	subjects, err := s.subjects.List(ctx, false)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]storage.Subject, len(subjects))
	for _, subject := range subjects {
		byID[subject.ID] = subject
	}

	return byID, nil
}
