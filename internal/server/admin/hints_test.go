package admin_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestAdminSectionsExplainThemselves(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	tests := []struct {
		path string
		hint string
	}{
		{"/admin/subjects", "чтобы в карточке класса назначить, кто что ведёт"},
		{"/admin/classes", "Как устроены классы"},
		{"/admin/teachers", "задаётся в карточке класса"},
		{"/admin/students", "Класс ученика можно указать сразу или позже"},
		{"/admin/parents", "Детей привязывают в строке правки"},
		{"/admin/substitutions", "Как работают замены"},
		{"/admin/journals", "Здесь можно только читать"},
		{"/admin/diary", "собирается из уроков его класса"},
		{"/admin/password-reset", "Пароль администратора меняется в конфиге сервера"},
		{"/admin/backup", "храните не на этом сервере"},
		{"/admin/penalties", "Что значат баллы, школа решает сама"},
	}

	for _, tt := range tests {
		recorder := servertest.Get(t, env.Handler, tt.path, admin)
		require.Equal(t, http.StatusOK, recorder.Code, tt.path)
		body := recorder.Body.String()
		assert.Contains(t, body, tt.hint, tt.path)

		details := strings.Index(body, "<details")
		assert.Positive(t, details, tt.path)
		assert.Less(t, details, strings.Index(body, tt.hint), "hint is collapsed: "+tt.path)
		assert.Less(t, strings.Index(body, tt.hint), strings.Index(body, "</details>"), "hint is collapsed: "+tt.path)
	}
}
