package admin_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/server/servertest"
)

func assignForm(subjectID, teacherID int64) url.Values {
	return url.Values{
		"subject_id": {strconv.FormatInt(subjectID, 10)},
		"teacher_id": {strconv.FormatInt(teacherID, 10)},
	}
}

func TestClassAssignmentsAssignReplaceRemove(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	class := createClass(t, env, admin, current, "7А")
	other := createClass(t, env, admin, current, "7Б")
	algebra := createSubject(t, env, admin, "Алгебра")
	history := createSubject(t, env, admin, "История")
	drawing := createSubject(t, env, admin, "Черчение")
	require.NoError(t, env.School.SetSubjectActive(t.Context(), drawing, false))
	anna, _ := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Сидорова Анна"}})
	viktor, _ := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Кузнецов Виктор"}})
	gone, _ := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Ушедший Учитель"}})
	require.NoError(t, env.Auth.SetUserActive(t.Context(), gone, false))
	student, _ := createUserVia(t, env, admin, "/admin/students", url.Values{"full_name": {"Кузнецов Олег"}})

	card := servertest.Get(t, env.Handler, classPathFor(class, ""), admin).Body.String()
	assert.Contains(t, card, "Предметы пока не назначены")
	assert.Contains(t, card, `action="`+classPathFor(class, "/assignments")+`"`)
	assert.Contains(t, card, `hx-target="#class-assignments"`)
	assert.Contains(t, card, `value="`+strconv.FormatInt(algebra, 10)+`">Алгебра<`)
	assert.Contains(t, card, `value="`+strconv.FormatInt(anna, 10)+`">Сидорова Анна<`)
	assert.NotContains(t, card, "Черчение")
	assert.NotContains(t, card, "Ушедший Учитель")
	assert.Contains(t, card, "Выберите предмет")
	assert.Contains(t, card, "Выберите учителя")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), assignForm(algebra, anna), []*http.Cookie{admin}, nil), classPathFor(class, ""))

	assignments, err := env.School.Assignments(t.Context(), class)
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, anna, assignments[0].TeacherID)
	first := assignments[0].ID

	card = servertest.Get(t, env.Handler, classPathFor(class, ""), admin).Body.String()
	assert.Contains(t, card, ">Алгебра</span>")
	assert.Contains(t, card, ">Сидорова Анна</span>")
	assert.Contains(t, card, `action="`+classPathFor(class, "/assignments/"+strconv.FormatInt(first, 10)+"/remove")+`"`)
	assert.NotContains(t, card, "Предметы пока не назначены")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), assignForm(algebra, viktor), []*http.Cookie{admin}, nil), classPathFor(class, ""))

	assignments, err = env.School.Assignments(t.Context(), class)
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, first, assignments[0].ID)
	assert.Equal(t, viktor, assignments[0].TeacherID)

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), assignForm(history, anna), []*http.Cookie{admin}, nil), classPathFor(class, ""))

	assignments, err = env.School.Assignments(t.Context(), class)
	require.NoError(t, err)
	require.Len(t, assignments, 2)

	cases := map[string]struct {
		form    url.Values
		message string
	}{
		"blank":            {url.Values{}, "Выберите предмет"},
		"blank teacher":    {assignForm(algebra, 0), "Выберите учителя"},
		"inactive subject": {assignForm(drawing, anna), "Такого предмета нет"},
		"unknown subject":  {assignForm(999, anna), "Такого предмета нет"},
		"inactive teacher": {assignForm(algebra, gone), "Такого учителя нет"},
		"student":          {assignForm(algebra, student), "Такого учителя нет"},
	}

	for name, tc := range cases {
		rejected := servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), tc.form, []*http.Cookie{admin}, nil)
		assert.Equal(t, http.StatusOK, rejected.Code, name)
		assert.Contains(t, rejected.Body.String(), "<html", name)
		assert.Contains(t, rejected.Body.String(), tc.message, name)
	}

	blank := servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), url.Values{}, []*http.Cookie{admin}, nil).Body.String()
	assert.Contains(t, blank, "Выберите учителя")

	assignments, err = env.School.Assignments(t.Context(), class)
	require.NoError(t, err)
	require.Len(t, assignments, 2)

	require.NoError(t, env.School.SetSubjectActive(t.Context(), algebra, false))
	require.NoError(t, env.Auth.SetUserActive(t.Context(), viktor, false))

	card = servertest.Get(t, env.Handler, classPathFor(class, ""), admin).Body.String()
	assert.Contains(t, card, ">Алгебра</span>")
	assert.Contains(t, card, ">Кузнецов Виктор</span>")
	assert.Equal(t, 2, strings.Count(card, "удалён"))

	remove := classPathFor(class, "/assignments/"+strconv.FormatInt(first, 10)+"/remove")
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, classPathFor(other, "/assignments/"+strconv.FormatInt(first, 10)+"/remove"), nil, []*http.Cookie{admin}, nil).Code)
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, remove, nil, []*http.Cookie{admin}, nil), classPathFor(class, ""))
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, remove, nil, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments/abc/remove"), nil, []*http.Cookie{admin}, nil).Code)

	assignments, err = env.School.Assignments(t.Context(), class)
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, history, assignments[0].SubjectID)
}

func TestClassAssignmentsHtmxFragment(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()
	htmx := map[string]string{"HX-Request": "true"}

	class := createClass(t, env, admin, current, "7А")
	algebra := createSubject(t, env, admin, "Алгебра")
	anna, _ := createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Сидорова Анна"}})

	assigned := servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), assignForm(algebra, anna), []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, assigned.Code)
	assert.NotContains(t, assigned.Body.String(), "<html")
	assert.Contains(t, assigned.Body.String(), `id="class-assignments"`)
	assert.NotContains(t, assigned.Body.String(), `id="class-students"`)
	assert.Contains(t, assigned.Body.String(), ">Сидорова Анна</span>")

	rejected := servertest.PostForm(t, env.Handler, classPathFor(class, "/assignments"), assignForm(algebra, 0), []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, rejected.Code)
	assert.NotContains(t, rejected.Body.String(), "<html")
	assert.Contains(t, rejected.Body.String(), `id="class-assignments"`)
	assert.Contains(t, rejected.Body.String(), "Выберите учителя")
}

func TestClassAssignmentsNeedSubjectsAndTeachers(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	class := createClass(t, env, admin, current, "7А")

	card := servertest.Get(t, env.Handler, classPathFor(class, ""), admin).Body.String()
	assert.Contains(t, card, "Нужны активные предметы и учителя")
	assert.NotContains(t, card, "Выберите предмет")

	createSubject(t, env, admin, "Алгебра")

	card = servertest.Get(t, env.Handler, classPathFor(class, ""), admin).Body.String()
	assert.Contains(t, card, "Нужны активные предметы и учителя")

	createUserVia(t, env, admin, "/admin/teachers", url.Values{"full_name": {"Сидорова Анна"}})

	card = servertest.Get(t, env.Handler, classPathFor(class, ""), admin).Body.String()
	assert.NotContains(t, card, "Нужны активные предметы и учителя")
	assert.Contains(t, card, "Выберите предмет")
}
