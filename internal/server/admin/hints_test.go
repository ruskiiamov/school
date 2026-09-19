package admin_test

import (
	"net/http"
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
		{"/admin/work-types", "за что поставлена оценка"},
		{"/admin/classes", "Как устроены классы"},
		{"/admin/teachers", "задаётся в карточке класса"},
		{"/admin/students", "Класс ученика можно указать сразу или позже"},
		{"/admin/parents", "Детей привязывают в строке правки"},
		{"/admin/substitutions", "Как работают замены"},
		{"/admin/journal", "Журналы никто не формирует"},
		{"/admin/diary", "собирается из уроков его класса"},
		{"/admin/password-reset", "Пароль администратора меняется в конфиге сервера"},
		{"/admin/backup", "храните не на этом сервере"},
	}

	for _, tt := range tests {
		recorder := servertest.Get(t, env.Handler, tt.path, admin)
		require.Equal(t, http.StatusOK, recorder.Code, tt.path)
		assert.Contains(t, recorder.Body.String(), tt.hint, tt.path)
	}
}
