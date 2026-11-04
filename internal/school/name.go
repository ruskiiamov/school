package school

import (
	"strings"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgNameRequired = "Укажите название"
	msgNameTooLong  = "Название длиннее 100 символов"
	msgNameTaken    = "Такое название уже есть среди активных"

	msgClassNameOneWord = "Название класса — одно слово без пробелов, например 5А: слово «класс» приложение допишет само"
)

// validateClassName — общая проверка названия плюс «одно слово»: в
// интерфейсе к названию дописывается «класс» (D-115).
func validateClassName(errs validation.Errors, name string) {
	validateName(errs, name)

	if _, failed := errs["name"]; !failed && strings.ContainsRune(name, ' ') {
		errs.Add("name", msgClassNameOneWord)
	}
}

func validateName(errs validation.Errors, name string) {
	switch {
	case name == "":
		errs.Add("name", msgNameRequired)
	case utf8.RuneCountInString(name) > maxNameLength:
		errs.Add("name", msgNameTooLong)
	}
}
