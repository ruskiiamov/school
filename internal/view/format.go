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
