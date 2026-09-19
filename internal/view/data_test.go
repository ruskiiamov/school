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
		{"admin", []string{"Главная", "Классы", "Учителя", "Ученики", "Родители", "Замены", "Предметы", "Типы работ", "Журналы", "Дневники", "Сброс пароля", "Резервная копия"}},
		{"teacher", []string{"Главная", "Журнал", "Сводка", "Сменить пароль"}},
		{"student", []string{"Главная", "Дневник", "Оценки", "Сменить пароль"}},
		{"parent", []string{"Главная", "Дневник", "Оценки", "Сменить пароль"}},
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

func TestNavItemsGroupAdminSections(t *testing.T) {
	t.Parallel()

	groups := map[string][]string{}
	for _, item := range NavItems("admin", "/") {
		groups[item.Group] = append(groups[item.Group], item.Title)
	}

	assert.Equal(t, map[string][]string{
		"":            {"Главная"},
		"Школа":       {"Классы", "Учителя", "Ученики", "Родители", "Замены"},
		"Справочники": {"Предметы", "Типы работ"},
		"Просмотр":    {"Журналы", "Дневники"},
		"Служебное":   {"Сброс пароля", "Резервная копия"},
	}, groups)

	for _, item := range NavItems("teacher", "/") {
		assert.Empty(t, item.Group, item.Title)
	}
}

func TestAdminSetupHidesWhenRequiredStepsDone(t *testing.T) {
	t.Parallel()

	steps := AdminSetup(SetupCounts{Subjects: 1})
	require.Len(t, steps, 6)
	assert.True(t, steps[0].Done)
	assert.False(t, steps[1].Done)
	assert.True(t, steps[5].Optional)

	assert.Nil(t, AdminSetup(SetupCounts{Subjects: 1, Classes: 1, Teachers: 1, Students: 1, Assignments: 1}))
}
