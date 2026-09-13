package school

import (
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgNameRequired = "Укажите название"
	msgNameTooLong  = "Название длиннее 100 символов"
	msgNameTaken    = "Такое название уже есть среди активных"
)

func validateName(errs validation.Errors, name string) {
	switch {
	case name == "":
		errs.Add("name", msgNameRequired)
	case utf8.RuneCountInString(name) > maxNameLength:
		errs.Add("name", msgNameTooLong)
	}
}
