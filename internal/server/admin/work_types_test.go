package admin_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

var seededWorkTypes = []string{
	"Ответ на уроке",
	"Классная работа",
	"Самостоятельная работа",
	"Контрольная работа",
	"Домашняя работа (письменная)",
	"Домашняя работа (устная)",
}

func workTypePath(id int64, suffix string) string {
	return "/admin/work-types/" + strconv.FormatInt(id, 10) + suffix
}

func workTypeByName(t *testing.T, env *servertest.Env, name string) school.WorkType {
	t.Helper()

	workTypes, err := env.School.WorkTypes(t.Context(), true)
	require.NoError(t, err)

	for _, workType := range workTypes {
		if workType.Name == name {
			return workType
		}
	}

	t.Fatalf("work type %q not found", name)

	return school.WorkType{}
}

func activeWorkTypeNames(t *testing.T, env *servertest.Env) []string {
	t.Helper()

	workTypes, err := env.School.WorkTypes(t.Context(), false)
	require.NoError(t, err)

	names := make([]string, 0, len(workTypes))
	for _, workType := range workTypes {
		names = append(names, workType.Name)
	}

	return names
}

func TestWorkTypesListShowsSeededTypesInOrder(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	recorder := servertest.Get(t, env.Handler, "/admin/work-types", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	last := -1

	for _, name := range seededWorkTypes {
		position := strings.Index(body, ">"+name+"</span>")
		assert.Greater(t, position, last, name)
		last = position
	}

	assert.Equal(t, seededWorkTypes, activeWorkTypeNames(t, env))

	first := workTypeByName(t, env, seededWorkTypes[0])
	assert.Equal(t, 10, first.SortOrder)
	assert.Contains(t, body, `formaction="`+workTypePath(first.ID, "/down")+`"`)
	assert.Contains(t, body, `formaction="`+workTypePath(first.ID, "/up")+`" hx-post="`+workTypePath(first.ID, "/up")+`" hx-target="#work-types" hx-swap="outerHTML" aria-label="Выше" title="Выше" disabled`)
	assert.Contains(t, body, `href="/admin/work-types?edit=`+strconv.FormatInt(first.ID, 10)+`"`)
	assert.NotContains(t, body, `aria-label="Название типа работы"`)
}

func TestWorkTypeCreateAppendsToEnd(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	created := servertest.PostForm(t, env.Handler, "/admin/work-types", url.Values{"name": {"Проект"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, created, "/admin/work-types")

	assert.Equal(t, append(append([]string{}, seededWorkTypes...), "Проект"), activeWorkTypeNames(t, env))

	project := workTypeByName(t, env, "Проект")
	assert.Equal(t, 70, project.SortOrder)
	assert.True(t, project.Active)
}

func TestWorkTypeMoveUpAndDown(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	control := workTypeByName(t, env, "Контрольная работа")
	first := workTypeByName(t, env, "Ответ на уроке")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(control.ID, "/up"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")
	assert.Equal(t, []string{
		"Ответ на уроке", "Классная работа", "Контрольная работа", "Самостоятельная работа",
		"Домашняя работа (письменная)", "Домашняя работа (устная)",
	}, activeWorkTypeNames(t, env))

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(first.ID, "/up"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")
	assert.Equal(t, "Ответ на уроке", activeWorkTypeNames(t, env)[0])

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(first.ID, "/down"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")
	assert.Equal(t, []string{"Классная работа", "Ответ на уроке"}, activeWorkTypeNames(t, env)[:2])

	workTypes, err := env.School.WorkTypes(t.Context(), true)
	require.NoError(t, err)

	for i, workType := range workTypes {
		assert.Equal(t, (i+1)*10, workType.SortOrder, workType.Name)
	}
}

func TestWorkTypeMoveSkipsInactiveNeighbour(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	classwork := workTypeByName(t, env, "Классная работа")
	independent := workTypeByName(t, env, "Самостоятельная работа")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(classwork.ID, "/deactivate"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(independent.ID, "/up"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")

	assert.Equal(t, []string{"Самостоятельная работа", "Ответ на уроке"}, activeWorkTypeNames(t, env)[:2])

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(classwork.ID, "/up"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")
	assert.Equal(t, []string{"Самостоятельная работа", "Ответ на уроке"}, activeWorkTypeNames(t, env)[:2])

	body := servertest.Get(t, env.Handler, "/admin/work-types?inactive=1", admin).Body.String()
	assert.NotContains(t, body, `formaction="`+workTypePath(classwork.ID, "/up?inactive=1")+`"`)
	assert.NotContains(t, body, `formaction="`+workTypePath(classwork.ID, "/activate?inactive=1")+`"`)
	assert.Contains(t, body, `href="/admin/work-types?edit=`+strconv.FormatInt(classwork.ID, 10)+`&amp;inactive=1"`)

	editing := servertest.Get(t, env.Handler, "/admin/work-types?edit="+strconv.FormatInt(classwork.ID, 10)+"&inactive=1", admin).Body.String()
	assert.Contains(t, editing, `formaction="`+workTypePath(classwork.ID, "/activate?inactive=1")+`"`)
	assert.NotContains(t, editing, `formaction="`+workTypePath(classwork.ID, "/up?inactive=1")+`"`)
}

func TestWorkTypeRenameAndValidation(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	workType := workTypeByName(t, env, "Классная работа")

	editing := servertest.Get(t, env.Handler, "/admin/work-types?edit="+strconv.FormatInt(workType.ID, 10), admin).Body.String()
	assert.Contains(t, editing, `value="Классная работа"`)
	assert.Equal(t, 1, strings.Count(editing, `aria-label="Название типа работы"`))

	updated := servertest.PostForm(t, env.Handler, workTypePath(workType.ID, ""), url.Values{"name": {"Работа в классе"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, updated, "/admin/work-types")

	changed, err := env.School.WorkTypeByID(t.Context(), workType.ID)
	require.NoError(t, err)
	assert.Equal(t, "Работа в классе", changed.Name)
	assert.Equal(t, 20, changed.SortOrder)

	tests := []struct {
		name    string
		form    url.Values
		message string
	}{
		{"taken name", url.Values{"name": {"Контрольная работа"}}, "уже есть среди активных"},
		{"empty name", url.Values{"name": {""}}, "Укажите название"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := servertest.PostForm(t, env.Handler, workTypePath(workType.ID, ""), tt.form, []*http.Cookie{admin}, nil)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), tt.message)
			assert.Contains(t, recorder.Body.String(), `value="`+tt.form.Get("name")+`"`)
			assert.Equal(t, 1, strings.Count(recorder.Body.String(), `aria-label="Название типа работы"`))
			assert.NotContains(t, recorder.Body.String(), "Работа в классе")
		})
	}

	unchanged, err := env.School.WorkTypeByID(t.Context(), workType.ID)
	require.NoError(t, err)
	assert.Equal(t, changed, unchanged)
}

func TestWorkTypeDeactivateAndActivate(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	workType := workTypeByName(t, env, "Домашняя работа (устная)")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, workTypePath(workType.ID, "/deactivate"), nil, []*http.Cookie{admin}, nil), "/admin/work-types")

	visible := servertest.Get(t, env.Handler, "/admin/work-types", admin).Body.String()
	assert.NotContains(t, visible, "Домашняя работа (устная)")

	all := servertest.Get(t, env.Handler, "/admin/work-types?inactive=1", admin).Body.String()
	assert.Contains(t, all, "Домашняя работа (устная)")
	assert.Contains(t, all, "удалён")

	reused := servertest.PostForm(t, env.Handler, "/admin/work-types", url.Values{"name": {"Домашняя работа (устная)"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, reused, "/admin/work-types")

	conflict := servertest.PostForm(t, env.Handler, workTypePath(workType.ID, "/activate?inactive=1"), nil, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "уже есть среди активных")

	assert.Len(t, activeWorkTypeNames(t, env), len(seededWorkTypes))
}

func TestWorkTypesHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	student := env.LoginAs(t, "student")
	admin := servertest.Login(t, env.Handler)

	first := workTypeByName(t, env, seededWorkTypes[0])

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/admin/work-types"), "/login")
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/work-types", student).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/work-types", url.Values{"name": {"Проект"}}, []*http.Cookie{student}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, workTypePath(first.ID, "/down"), nil, []*http.Cookie{student}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/work-types/999/up", nil, []*http.Cookie{admin}, nil).Code)

	assert.Equal(t, seededWorkTypes, activeWorkTypeNames(t, env))
}
