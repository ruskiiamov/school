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

func TestFormatFileSize(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "512 Б", FormatFileSize(512))
	assert.Equal(t, "34 КБ", FormatFileSize(34*1024+100))
	assert.Equal(t, "1,3 МБ", FormatFileSize(1_400_000))
}

func TestPlural(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1 файл", Plural(1, "файл", "файла", "файлов"))
	assert.Equal(t, "3 файла", Plural(3, "файл", "файла", "файлов"))
	assert.Equal(t, "11 файлов", Plural(11, "файл", "файла", "файлов"))
	assert.Equal(t, "22 файла", Plural(22, "файл", "файла", "файлов"))
	assert.Equal(t, "5 файлов", Plural(5, "файл", "файла", "файлов"))
}
