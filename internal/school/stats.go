package school

import "context"

type Stats struct {
	Year        int
	Classes     int
	Students    int
	Subjects    int
	Assignments int
}

func (s *Service) Stats(ctx context.Context) (Stats, error) {
	year := s.CurrentYear()

	classes, err := s.classes.CountActiveByYear(ctx, year)
	if err != nil {
		return Stats{}, err
	}

	students, err := s.classStudents.CountActiveByYear(ctx, year)
	if err != nil {
		return Stats{}, err
	}

	subjects, err := s.subjects.CountActive(ctx)
	if err != nil {
		return Stats{}, err
	}

	assignments, err := s.assignments.CountByYear(ctx, year)
	if err != nil {
		return Stats{}, err
	}

	return Stats{Year: year, Classes: classes, Students: students, Subjects: subjects, Assignments: assignments}, nil
}
