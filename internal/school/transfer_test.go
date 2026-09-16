package school_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/school"
)

func TestNextClassName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"7А":               "8А",
		"10Б":              "11Б",
		"9":                "10",
		"Подготовительный": "Подготовительный",
		"":                 "",
	}

	for name, want := range tests {
		assert.Equal(t, want, school.NextClassName(name), name)
	}
}
