package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuggestLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input Name
		want  string
	}{
		{"surname and initial", Name{Last: "Иванова", First: "Мария", Middle: "Петровна"}, "ivanova.m"},
		{"multi-letter transliteration", Name{Last: "Щукин", First: "Ёж"}, "shchukin.yo"},
		{"soft sign and short i dropped", Name{Last: "Соловьёв", First: "Юрий"}, "solovyov.yu"},
		{"surname only", Name{Last: "Кузнецов"}, "kuznetsov"},
		{"latin and digits kept lowercase", Name{Last: "Smith", First: "John-2"}, "smith.j"},
		{"double surname", Name{Last: "Петров-Водкин", First: "Иван"}, "petrovvodkin.i"},
		{"empty", Name{}, "user"},
		{"nothing to transliterate", Name{Last: "---", First: "???"}, "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, SuggestLogin(tt.input))
		})
	}
}
