package view

import (
	"fmt"
	"time"
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

func RoleTitle(role string) string {
	if title, ok := roleTitles[role]; ok {
		return title
	}

	return role
}
