package web

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
)

func SelectYear(r *http.Request, years []int, fallback int) int {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || !slices.Contains(years, year) {
		return fallback
	}

	return year
}

func YearSelection(r *http.Request, s *school.Service) (int, []view.Option, error) {
	years, err := s.ClassYears(r.Context())
	if err != nil {
		return 0, nil, err
	}

	year := SelectYear(r, years, s.CurrentYear())

	return year, YearOptions(years, year), nil
}

func YearOptions(years []int, selected int) []view.Option {
	options := make([]view.Option, 0, len(years))
	for _, year := range years {
		options = append(options, view.Option{ID: int64(year), Name: school.YearName(year), Selected: year == selected})
	}

	return options
}

func WithYear(hidden map[string]string, year int) map[string]string {
	result := make(map[string]string, len(hidden)+1)
	for name, value := range hidden {
		result[name] = value
	}

	result["year"] = strconv.Itoa(year)

	return result
}

func YearPeriod(r *http.Request, s *school.Service, year int) (Period, time.Time) {
	start, end := s.YearBounds(year)

	anchor := s.Today()
	if year != s.CurrentYear() {
		anchor = start
	}

	return ParsePeriod(r, anchor, start, end), anchor
}
