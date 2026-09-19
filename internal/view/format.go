package view

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	monthsGenitive = [...]string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}

	roleTitles = map[string]string{
		"admin":   "Администратор",
		"teacher": "Учитель",
		"student": "Ученик",
		"parent":  "Родитель",
	}

	weekdays = map[time.Weekday]string{
		time.Monday:    "понедельник",
		time.Tuesday:   "вторник",
		time.Wednesday: "среда",
		time.Thursday:  "четверг",
		time.Friday:    "пятница",
		time.Saturday:  "суббота",
		time.Sunday:    "воскресенье",
	}
)

func FormatDate(t time.Time) string {
	return fmt.Sprintf("%s, %d %s %d", weekdays[t.Weekday()], t.Day(), monthsGenitive[t.Month()-1], t.Year())
}

func FormatShortDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func FormatPeriod(start, end time.Time) string {
	if end.IsZero() {
		return FormatShortDate(start) + " — до отмены"
	}

	return FormatShortDate(start) + " — " + FormatShortDate(end)
}

func ShortName(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) < 2 {
		return fullName
	}

	short := parts[0]

	for _, part := range parts[1:] {
		initial, _ := utf8.DecodeRuneInString(part)
		short += " " + string(initial) + "."
	}

	return short
}

func RoleTitle(role string) string {
	if title, ok := roleTitles[role]; ok {
		return title
	}

	return role
}

func FormatFileSize(size int64) string {
	const (
		kilobyte = 1 << 10
		megabyte = 1 << 20
	)

	switch {
	case size >= megabyte:
		return strings.Replace(fmt.Sprintf("%.1f МБ", float64(size)/megabyte), ".", ",", 1)
	case size >= kilobyte:
		return fmt.Sprintf("%d КБ", size/kilobyte)
	default:
		return fmt.Sprintf("%d Б", size)
	}
}

func Plural(n int, one, few, many string) string {
	mod10, mod100 := n%10, n%100

	switch {
	case mod10 == 1 && mod100 != 11:
		return fmt.Sprintf("%d %s", n, one)
	case mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20):
		return fmt.Sprintf("%d %s", n, few)
	default:
		return fmt.Sprintf("%d %s", n, many)
	}
}

func FormatAverage(average float64, count int) string {
	if count == 0 {
		return "—"
	}

	return strings.Replace(fmt.Sprintf("%.2f", average), ".", ",", 1)
}
