package server_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestHomeShowsNoClassesBannerForAdmin(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	body := servertest.Get(t, env.Handler, "/", admin).Body.String()
	yearName := school.YearName(env.School.CurrentYear())

	assert.Contains(t, body, "В "+yearName+" ещё нет классов")
	assert.Contains(t, body, "Создать класс")
	assert.Contains(t, body, "Учебный год "+yearName)
	assert.NotContains(t, body, "Раздел в разработке")
}

func TestHomeCountsCurrentYearForAdmin(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	ctx := t.Context()

	classID, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)

	_, err = env.School.CreateClass(ctx, "8Б")
	require.NoError(t, err)

	deleted, err := env.School.CreateClass(ctx, "9В")
	require.NoError(t, err)
	require.NoError(t, env.School.SetClassActive(ctx, deleted, false))

	_, err = env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	inactiveSubject, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Черчение"})
	require.NoError(t, err)
	require.NoError(t, env.School.SetSubjectActive(ctx, inactiveSubject, false))

	for _, name := range []string{"Сидорова Анна", "Петров Иван", "Уволенный Учитель"} {
		created, err := env.Auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleTeacher, FullName: name})
		require.NoError(t, err)

		if name == "Уволенный Учитель" {
			require.NoError(t, env.Auth.SetUserActive(ctx, created.User.ID, false))
		}
	}

	env.CreateUser(t, auth.RoleStudent, "free", "Без Класса")

	for _, name := range []string{"Козлов Пётр", "Иванова Мария", "Выбывший Ученик"} {
		created, err := env.Auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleStudent, FullName: name})
		require.NoError(t, err)
		require.NoError(t, env.School.SetStudentClass(ctx, created.User.ID, classID))

		if name == "Выбывший Ученик" {
			require.NoError(t, env.Auth.SetUserActive(ctx, created.User.ID, false))
		}
	}

	body := servertest.Get(t, env.Handler, "/", admin).Body.String()

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

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	env.CreateUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	env.CreateUser(t, auth.RoleParent, "parent", "Петрова Ольга Николаевна")

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
			cookie := env.LoginAs(t, tt.login)

			recorder := servertest.Get(t, env.Handler, "/", cookie)
			require.Equal(t, http.StatusOK, recorder.Code)

			body := recorder.Body.String()
			assert.Contains(t, body, "Здравствуйте, ")
			assert.Contains(t, body, `href="`+tt.href+`"`)
			assert.Contains(t, body, tt.title)
			assert.Contains(t, body, "Раздел в разработке")
			assert.NotContains(t, body, "Учебный год")
			assert.NotContains(t, body, `href="/admin/classes"`)
			assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/classes", cookie).Code)
			assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/students", cookie).Code)
		})
	}

}
