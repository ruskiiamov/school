package validation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/validation"
)

func TestParseDate(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)

	for _, value := range []string{"05.09.2026", "5.9.2026", "2026-09-05"} {
		parsed, ok := validation.ParseDate(value)
		assert.True(t, ok, value)
		assert.True(t, want.Equal(parsed), value)
	}

	for _, value := range []string{"", "31.02.2026", "05.09.26", "09/05/2026", "завтра"} {
		_, ok := validation.ParseDate(value)
		assert.False(t, ok, value)
	}
}
