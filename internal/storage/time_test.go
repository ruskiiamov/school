package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMillisRoundTrip(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 8, 31, 10, 0, 0, 123000000, time.UTC)

	got := fromMillis(toMillis(want))

	assert.True(t, want.Equal(got))
	assert.Equal(t, time.UTC, got.Location())
}

func TestMillisTruncateSubMillisecond(t *testing.T) {
	t.Parallel()

	withNanos := time.Date(2026, 8, 31, 10, 0, 0, 123456789, time.UTC)

	got := fromMillis(toMillis(withNanos))

	assert.Equal(t, time.Date(2026, 8, 31, 10, 0, 0, 123000000, time.UTC), got)
}

func TestMillisPreserveOrder(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)

	moments := []time.Time{
		base,
		base.Add(time.Millisecond),
		base.Add(time.Second),
		base.Add(time.Hour),
		base.AddDate(0, 0, 1),
	}

	for i := 1; i < len(moments); i++ {
		assert.Less(t, toMillis(moments[i-1]), toMillis(moments[i]))
	}
}

func TestMillisIndependentOfLocation(t *testing.T) {
	t.Parallel()

	moscow := time.FixedZone("MSK", 3*60*60)
	utc := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)

	assert.Equal(t, toMillis(utc), toMillis(utc.In(moscow)))
}
