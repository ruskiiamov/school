package school

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestYearAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		date time.Time
		want int
	}{
		{"first day of start month", time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), 2026},
		{"day before start month", time.Date(2026, time.July, 31, 23, 59, 0, 0, time.UTC), 2025},
		{"september", time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), 2026},
		{"spring of the same year", time.Date(2027, time.March, 15, 0, 0, 0, 0, time.UTC), 2026},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, YearAt(tt.date, time.August))
		})
	}
}

func TestCurrentYearUsesConfiguredLocation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	svc := NewService(now.Month(), time.UTC, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	assert.Equal(t, YearAt(now.UTC(), now.Month()), svc.CurrentYear())
}

func TestYearName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "2026/2027", YearName(2026))
}
