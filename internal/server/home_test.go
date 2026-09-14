package server

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
)

func TestHomeShowsNoClassesBannerForAdmin(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)

	body := get(t, env.handler, "/", admin).Body.String()
	yearName := school.YearName(env.school.CurrentYear())

	assert.Contains(t, body, "В "+yearName+" ещё нет классов")
	assert.Contains(t, body, "Создать класс")
	assert.Contains(t, body, "Учебный год "+yearName)
	assert.NotContains(t, body, "Раздел в разработке")
}

func TestHomeCountsCurrentYearForAdmin(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	ctx := t.Context()

	classID, err := env.school.CreateClass(ctx, "7А")
	require.NoError(t, err)

	_, err = env.school.CreateClass(ctx, "8Б")
	require.NoError(t, err)

	deleted, err := env.school.CreateClass(ctx, "9В")
	require.NoError(t, err)
	require.NoError(t, env.school.SetClassActive(ctx, deleted, false))

	_, err = env.school.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	inactiveSubject, err := env.school.CreateSubject(ctx, school.SubjectInput{Name: "Черчение"})
	require.NoError(t, err)
	require.NoError(t, env.school.SetSubjectActive(ctx, inactiveSubject, false))

	for _, name := range []string{"Сидорова Анна", "Петров Иван", "Уволенный Учитель"} {
		created, err := env.auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleTeacher, FullName: name})
		require.NoError(t, err)

		if name == "Уволенный Учитель" {
			require.NoError(t, env.auth.SetUserActive(ctx, created.User.ID, false))
		}
	}

	env.createUser(t, auth.RoleStudent, "free", "Без Класса")

	for _, name := range []string{"Козлов Пётр", "Иванова Мария", "Выбывший Ученик"} {
		created, err := env.auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleStudent, FullName: name})
		require.NoError(t, err)
		require.NoError(t, env.school.SetStudentClass(ctx, created.User.ID, classID))

		if name == "Выбывший Ученик" {
			require.NoError(t, env.auth.SetUserActive(ctx, created.User.ID, false))
		}
	}

	body := get(t, env.handler, "/", admin).Body.String()

	assert.NotContains(t, body, "ещё нет классов")
	assertStat(t, body, "Классы", 2)
	assertStat(t, body, "Ученики", 2)
	assertStat(t, body, "Учителя", 2)
	assertStat(t, body, "Предметы", 1)
	assert.Contains(t, body, `href="/admin/students"`)
}

func assertStat(t *testing.T, body, label string, value int) {
	t.Helper()

	assert.Contains(t, body, label+`</span></span> <span class="mt-2 block text-2xl font-semibold tabular-nums text-slate-900 lg:text-3xl">`+strconv.Itoa(value)+`</span>`, label)
}

func TestHomeShowsSectionLinkForOtherRoles(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.createUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	env.createUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	env.createUser(t, auth.RoleParent, "parent", "Петрова Ольга Николаевна")

	tests := []struct {
		login string
		href  string
		title string
	}{
		{"teacher", "/journal", "Журнал"},
		{"student", "/diary", "Дневник"},
		{"parent", "/diary", "Дневник"},
	}

	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			cookie := env.loginAs(t, tt.login)

			recorder := get(t, env.handler, "/", cookie)
			require.Equal(t, http.StatusOK, recorder.Code)

			body := recorder.Body.String()
			assert.Contains(t, body, "Здравствуйте, ")
			assert.Contains(t, body, `href="`+tt.href+`"`)
			assert.Contains(t, body, tt.title)
			assert.Contains(t, body, "Раздел в разработке")
			assert.NotContains(t, body, "Учебный год")
			assert.NotContains(t, body, `href="/admin/classes"`)
			assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/admin/classes", cookie).Code)
			assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/admin/students", cookie).Code)
		})
	}

}
