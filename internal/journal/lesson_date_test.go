package journal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseLessonDateWithoutYear(t *testing.T) {
	t.Parallel()

	bounds := YearBounds{
		Start: time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2027, time.August, 31, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		raw  string
		want time.Time
		ok   bool
	}{
		{"04.11", time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC), true},
		{"4.11", time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC), true},
		{"01.09", time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), true},
		{"31.12", time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC), true},
		{"15.01", time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC), true},
		{"31.08", time.Date(2027, time.August, 31, 0, 0, 0, 0, time.UTC), true},
		{"04.11.2026", time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC), true},
		{"2027-03-02", time.Date(2027, time.March, 2, 0, 0, 0, 0, time.UTC), true},
		{"29.02", time.Time{}, false},
		{"31.11", time.Time{}, false},
		{"04", time.Time{}, false},
		{"", time.Time{}, false},
	}

	for _, tt := range tests {
		got, ok := parseLessonDate(tt.raw, bounds)
		assert.Equal(t, tt.ok, ok, tt.raw)
		assert.Equal(t, tt.want, got, tt.raw)
	}
}
