package validation

import (
	"sort"
	"strings"
)

type Errors map[string]string

func (e Errors) Add(field, message string) {
	e[field] = message
}

func (e Errors) Error() string {
	fields := make([]string, 0, len(e))
	for field := range e {
		fields = append(fields, field)
	}

	sort.Strings(fields)

	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field+": "+e[field])
	}

	return "validation: " + strings.Join(parts, "; ")
}
