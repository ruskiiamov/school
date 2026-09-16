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

func NormalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func NormalizeLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")

	var (
		result []string
		blank  int
	)

	for _, line := range lines {
		line = NormalizeSpaces(line)
		if line == "" {
			blank++
			continue
		}

		if len(result) > 0 && blank > 0 {
			result = append(result, "")
		}

		blank = 0
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
