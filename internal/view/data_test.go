package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNavItemsDependOnRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role   string
		titles []string
	}{
		{"admin", []string{"Дашборд", "Классы", "Предметы", "Учителя", "Ученики", "Родители", "Типы работ"}},
		{"teacher", []string{"Дашборд", "Журнал"}},
		{"student", []string{"Дашборд", "Дневник"}},
		{"parent", []string{"Дашборд", "Дневник"}},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()

			items := NavItems(tt.role, "/")
			require.Len(t, items, len(tt.titles))

			for i, item := range items {
				assert.Equal(t, tt.titles[i], item.Title)
				assert.NotEmpty(t, item.Href, item.Title)
				assert.Equal(t, i == 0, item.Active, item.Title)
			}
		})
	}
}

func TestNavItemsMarkActiveByHref(t *testing.T) {
	t.Parallel()

	items := NavItems("admin", "/admin/classes")

	assert.False(t, items[0].Active)
	assert.True(t, items[1].Active)
}
