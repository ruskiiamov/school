package school

import (
	"strconv"
	"time"
)

func (s *Service) CurrentYear() int {
	return YearAt(time.Now().In(s.location), s.yearStartMonth)
}

func (s *Service) Today() time.Time {
	return DateOf(time.Now().In(s.location))
}

func DateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func YearAt(t time.Time, startMonth time.Month) int {
	if t.Month() < startMonth {
		return t.Year() - 1
	}

	return t.Year()
}

func YearName(year int) string {
	return strconv.Itoa(year) + "/" + strconv.Itoa(year+1)
}
