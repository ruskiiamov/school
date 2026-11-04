package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestPenaltySettingsDefaults(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	recorder := servertest.Get(t, env.Handler, "/admin/penalties", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, `value="Штраф"`)
	assert.Contains(t, body, `value="баллы"`)
	assert.Contains(t, body, `value="20"`)
	assert.NotContains(t, body, "checked")
	assert.NotContains(t, body, `href="/penalties"`)
}

func TestPenaltySettingsSave(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	recorder := servertest.PostForm(t, env.Handler, "/admin/penalties",
		url.Values{"enabled": {"1"}, "name": {"  Мытьё   полов "}, "unit": {"м²"}, "full": {"15"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, recorder, "/admin/penalties?done=1")

	assert.Equal(t, school.PenaltySettings{Enabled: true, Name: "Мытьё полов", Unit: "м²", Full: 15}, env.School.PenaltySettings())

	body := servertest.Get(t, env.Handler, "/admin/penalties?done=1", admin).Body.String()
	assert.Contains(t, body, "Сохранено")
	assert.Contains(t, body, `href="/penalties"`)
	assert.Contains(t, body, "Мытьё полов")

	require.NoError(t, env.School.LoadSettings(t.Context()))
	assert.Equal(t, school.PenaltySettings{Enabled: true, Name: "Мытьё полов", Unit: "м²", Full: 15}, env.School.PenaltySettings())
}

func TestPenaltySettingsErrors(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	tests := []struct {
		name    string
		form    url.Values
		message string
	}{
		{"empty name", url.Values{"name": {""}, "unit": {"м²"}, "full": {"20"}}, "Введите название"},
		{"long name", url.Values{"name": {"Очень длинное название штрафа ш"}, "unit": {"м²"}, "full": {"20"}}, "Название: не больше 30 символов"},
		{"empty unit", url.Values{"name": {"Штраф"}, "unit": {""}, "full": {"20"}}, "Введите единицу"},
		{"long unit", url.Values{"name": {"Штраф"}, "unit": {"квадратных метров пол"}, "full": {"20"}}, "Единица: не больше 20 символов"},
		{"zero", url.Values{"name": {"Штраф"}, "unit": {"м²"}, "full": {"0"}}, "Полный штраф: целое число от 1 до 1000"},
		{"negative", url.Values{"name": {"Штраф"}, "unit": {"м²"}, "full": {"-5"}}, "Полный штраф: целое число от 1 до 1000"},
		{"too big", url.Values{"name": {"Штраф"}, "unit": {"м²"}, "full": {"1001"}}, "Полный штраф: целое число от 1 до 1000"},
		{"fraction", url.Values{"name": {"Штраф"}, "unit": {"м²"}, "full": {"7,5"}}, "Полный штраф: целое число от 1 до 1000"},
		{"text", url.Values{"name": {"Штраф"}, "unit": {"м²"}, "full": {"abc"}}, "Полный штраф: целое число от 1 до 1000"},
	}

	for _, tt := range tests {
		recorder := servertest.PostForm(t, env.Handler, "/admin/penalties", tt.form, []*http.Cookie{admin}, nil)
		require.Equal(t, http.StatusOK, recorder.Code, tt.name)
		assert.Contains(t, recorder.Body.String(), tt.message, tt.name)
	}

	assert.Equal(t, school.DefaultPenaltySettings(), env.School.PenaltySettings())
}

func TestPenaltySettingsOnlyForAdmin(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.LoginAs(t, "teacher")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/penalties", teacher).Code)

	recorder := servertest.PostForm(t, env.Handler, "/admin/penalties", url.Values{"enabled": {"1"}, "name": {"Штраф"}, "unit": {"баллы"}, "full": {"20"}},
		[]*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.False(t, env.School.PenaltySettings().Enabled)
}
