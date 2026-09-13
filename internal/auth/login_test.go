package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuggestLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fullName string
		want     string
	}{
		{"surname and initial", "Иванова Мария Петровна", "ivanova.m"},
		{"multi-letter transliteration", "Щукин Ёж", "shchukin.yo"},
		{"soft sign and short i dropped", "Соловьёв Юрий", "solovyov.yu"},
		{"single word", "Кузнецов", "kuznetsov"},
		{"latin and digits kept lowercase", "Smith John-2", "smith.j"},
		{"extra spaces", "  Петров   Иван ", "petrov.i"},
		{"empty", "   ", "user"},
		{"nothing to transliterate", "--- ???", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, SuggestLogin(tt.fullName))
		})
	}
}
