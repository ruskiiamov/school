package server

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/storage"
)

func studentForm(id int64) url.Values {
	return url.Values{"student_id": {strconv.FormatInt(id, 10)}}
}

func TestClassStudentsAddAndRemove(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	classA := createClass(t, env, admin, current, "7А")
	classB := createClass(t, env, admin, current, "7Б")
	free, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Кузнецов Олег"}})
	inB, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Петров Иван"}, "class": {strconv.FormatInt(classB, 10)}})
	gone, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Смирнова Мария"}})
	require.NoError(t, env.auth.SetUserActive(t.Context(), gone, false))
	teacher, _ := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Сидорова Анна"}})

	card := get(t, env.handler, classPathFor(classA, ""), admin)
	require.Equal(t, http.StatusOK, card.Code)
	assert.Contains(t, card.Body.String(), "Пока нет учеников")
	assert.Contains(t, card.Body.String(), `action="`+classPathFor(classA, "/students")+`"`)
	assert.Contains(t, card.Body.String(), `hx-target="#class-students"`)
	assert.Contains(t, card.Body.String(), "Выберите ученика")
	assert.Contains(t, card.Body.String(), `value="`+strconv.FormatInt(free, 10)+`">Кузнецов Олег<`)
	assert.NotContains(t, card.Body.String(), "Петров Иван")
	assert.NotContains(t, card.Body.String(), "Смирнова Мария")
	assert.NotContains(t, card.Body.String(), "required")

	assertRedirect(t, postForm(t, env.handler, classPathFor(classA, "/students"), studentForm(free), []*http.Cookie{admin}, nil), classPathFor(classA, ""))

	class, found, err := env.school.StudentClass(t.Context(), free)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, classA, class.ID)

	remove := classPathFor(classA, "/students/"+strconv.FormatInt(free, 10)+"/remove")

	card = get(t, env.handler, classPathFor(classA, ""), admin)
	assert.Contains(t, card.Body.String(), "Кузнецов Олег")
	assert.Contains(t, card.Body.String(), `action="`+remove+`"`)
	assert.Contains(t, card.Body.String(), "Нет учеников без класса")
	assert.NotContains(t, card.Body.String(), "Пока нет учеников")

	students := get(t, env.handler, "/admin/students?class="+strconv.FormatInt(classA, 10), admin)
	assert.Contains(t, students.Body.String(), "Кузнецов Олег")
	assert.NotContains(t, students.Body.String(), "Петров Иван")

	taken := postForm(t, env.handler, classPathFor(classA, "/students"), studentForm(inB), []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.Contains(t, taken.Body.String(), "<html")
	assert.Contains(t, taken.Body.String(), "Ученик уже в классе 7Б")

	for name, form := range map[string]url.Values{
		"teacher":  studentForm(teacher),
		"inactive": studentForm(gone),
		"garbage":  {"student_id": {"abc"}},
		"unknown":  studentForm(999),
	} {
		rejected := postForm(t, env.handler, classPathFor(classA, "/students"), form, []*http.Cookie{admin}, nil)
		assert.Equal(t, http.StatusOK, rejected.Code, name)
		assert.Contains(t, rejected.Body.String(), "Такого ученика нет", name)
	}

	classes, err := env.school.StudentClasses(t.Context(), current)
	require.NoError(t, err)
	assert.Len(t, classes, 2)

	assertRedirect(t, postForm(t, env.handler, remove, nil, []*http.Cookie{admin}, nil), classPathFor(classA, ""))

	_, found, err = env.school.StudentClass(t.Context(), free)
	require.NoError(t, err)
	assert.False(t, found)

	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, remove, nil, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, classPathFor(classA, "/students/abc/remove"), nil, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, classPathFor(999, "/students"), studentForm(free), []*http.Cookie{admin}, nil).Code)
}

func TestClassStudentsHtmxFragment(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()
	htmx := map[string]string{"HX-Request": "true"}

	classA := createClass(t, env, admin, current, "7А")
	classB := createClass(t, env, admin, current, "7Б")
	free, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Кузнецов Олег"}})
	inB, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Петров Иван"}, "class": {strconv.FormatInt(classB, 10)}})

	added := postForm(t, env.handler, classPathFor(classA, "/students"), studentForm(free), []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, added.Code)
	assert.NotContains(t, added.Body.String(), "<html")
	assert.Contains(t, added.Body.String(), `id="class-students"`)
	assert.NotContains(t, added.Body.String(), `id="class-assignments"`)
	assert.Contains(t, added.Body.String(), "Кузнецов Олег")

	taken := postForm(t, env.handler, classPathFor(classA, "/students"), studentForm(inB), []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.NotContains(t, taken.Body.String(), "<html")
	assert.Contains(t, taken.Body.String(), `id="class-students"`)
	assert.Contains(t, taken.Body.String(), "Ученик уже в классе 7Б")

	removed := postForm(t, env.handler, classPathFor(classA, "/students/"+strconv.FormatInt(free, 10)+"/remove"), nil, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, removed.Code)
	assert.NotContains(t, removed.Body.String(), "<html")
	assert.Contains(t, removed.Body.String(), "Пока нет учеников")
}

func TestClassStudentsReadOnlyOutsideCurrentYear(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	current := env.school.CurrentYear()

	past := createPastClass(t, env, current-1, "6А")
	closed := createClass(t, env, admin, current, "7А")
	require.NoError(t, env.school.SetClassActive(t.Context(), closed, false))
	student, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Кузнецов Олег"}})
	require.NoError(t, storage.NewClassStudentRepo(env.db).Add(t.Context(), past, student))

	for _, id := range []int64{past, closed} {
		card := get(t, env.handler, classPathFor(id, ""), admin)
		require.Equal(t, http.StatusOK, card.Code)
		assert.NotContains(t, card.Body.String(), `action="`+classPathFor(id, "/students")+`"`)
		assert.NotContains(t, card.Body.String(), `action="`+classPathFor(id, "/assignments")+`"`)
		assert.NotContains(t, card.Body.String(), "Убрать")
		assert.NotContains(t, card.Body.String(), "Выберите ученика")

		forged := postForm(t, env.handler, classPathFor(id, "/students"), studentForm(student), []*http.Cookie{admin}, nil)
		assert.Equal(t, http.StatusOK, forged.Code)
		assert.Contains(t, forged.Body.String(), "Такого класса нет в текущем году")

		removed := postForm(t, env.handler, classPathFor(id, "/students/"+strconv.FormatInt(student, 10)+"/remove"), nil, []*http.Cookie{admin}, nil)
		assert.Equal(t, http.StatusOK, removed.Code)
		assert.Contains(t, removed.Body.String(), "Такого класса нет в текущем году")
	}

	pastCard := get(t, env.handler, classPathFor(past, ""), admin).Body.String()
	assert.Contains(t, pastCard, "Кузнецов Олег")
	assert.NotContains(t, pastCard, "Пока нет учеников")

	_, found, err := env.school.StudentClass(t.Context(), student)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestClassCardActionsHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	admin := login(t, env.handler)
	env.createUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.loginAs(t, "teacher")
	current := env.school.CurrentYear()

	class := createClass(t, env, admin, current, "7А")
	student, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Кузнецов Олег"}})

	for _, path := range []string{
		classPathFor(class, "/students"),
		classPathFor(class, "/students/"+strconv.FormatInt(student, 10)+"/remove"),
		classPathFor(class, "/assignments"),
		classPathFor(class, "/assignments/1/remove"),
	} {
		assertRedirect(t, postForm(t, env.handler, path, studentForm(student), nil, nil), "/login")
		assert.Equal(t, http.StatusNotFound, postForm(t, env.handler, path, studentForm(student), []*http.Cookie{teacher}, nil).Code, path)
	}

	classes, err := env.school.StudentClasses(t.Context(), current)
	require.NoError(t, err)
	assert.Empty(t, classes)
}
