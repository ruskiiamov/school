package web_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/server/web"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestParsePeriod(t *testing.T) {
	t.Parallel()

	today := date(2026, time.September, 16)
	start, end := date(2026, time.August, 1), date(2027, time.July, 31)

	tests := []struct {
		name  string
		query string
		from  time.Time
		to    time.Time
	}{
		{name: "default is current month", query: "", from: date(2026, time.September, 1), to: date(2026, time.September, 30)},
		{name: "explicit", query: "?from=2026-09-05&to=2026-09-10", from: date(2026, time.September, 5), to: date(2026, time.September, 10)},
		{name: "swapped", query: "?from=2026-09-10&to=2026-09-05", from: date(2026, time.September, 5), to: date(2026, time.September, 10)},
		{name: "clamped to year", query: "?from=2026-01-01&to=2028-01-01", from: start, to: end},
		{name: "broken date falls back", query: "?from=2026-99-01&to=2026-09-10", from: date(2026, time.September, 1), to: date(2026, time.September, 30)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			period := web.ParsePeriod(httptest.NewRequest("GET", "/x"+tt.query, nil), today, start, end)
			assert.Equal(t, tt.from, period.From)
			assert.Equal(t, tt.to, period.To)
		})
	}
}

func TestPeriodMonthNavigation(t *testing.T) {
	t.Parallel()

	today := date(2026, time.August, 16)
	start, end := date(2026, time.August, 1), date(2027, time.July, 31)
	period := web.ParsePeriod(httptest.NewRequest("GET", "/x", nil), today, start, end)

	prev := period.Month(-1)
	assert.Equal(t, start, prev.From)
	assert.Equal(t, start, prev.To)

	next := period.Month(1)
	assert.Equal(t, date(2026, time.September, 1), next.From)
	assert.Equal(t, date(2026, time.September, 30), next.To)

	form := web.PeriodForm(period, today, "/journal/summary", map[string]string{"pair": "1-2"})
	assert.Equal(t, "2026-08-01", form.From)
	assert.Equal(t, "2026-08-31", form.To)
	assert.Equal(t, "01.08.2026 — 31.08.2026", form.Label)
	assert.Equal(t, "/journal/summary?from=2026-09-01&pair=1-2&to=2026-09-30", form.NextHref)
	assert.Equal(t, "/journal/summary?from=2026-08-01&pair=1-2&to=2026-08-31", form.ThisHref)
}
