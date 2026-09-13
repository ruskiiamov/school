package view

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "понедельник, 31 августа 2026",
		FormatDate(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)))
	assert.Equal(t, "четверг, 1 января 2026",
		FormatDate(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)))
}

func TestRoleTitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Администратор", RoleTitle("admin"))
	assert.Equal(t, "Учитель", RoleTitle("teacher"))
	assert.Equal(t, "unknown", RoleTitle("unknown"))
}
