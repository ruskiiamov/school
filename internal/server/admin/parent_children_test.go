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
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func parentsURL(parentID int64, child string) string {
	values := url.Values{"edit": {strconv.FormatInt(parentID, 10)}}
	if child != "" {
		values.Set("child", child)
	}

	return "/admin/parents?" + values.Encode()
}

func childrenPath(parentID int64, suffix, child string) string {
	return userPathFor("/admin/parents", parentID, "/children"+suffix) + strings.TrimPrefix(parentsURL(parentID, child), "/admin/parents")
}

func attr(href string) string {
	return strings.ReplaceAll(href, "&", "&amp;")
}

func TestParentChildrenInEditRow(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()

	class := createClass(t, env, admin, current, "7А")
	parent, _ := createUserVia(t, env, admin, "/admin/parents", servertest.NameForm("Иванова Мария"))
	other, _ := createUserVia(t, env, admin, "/admin/parents", servertest.NameForm("Иванов Сергей"))
	petr, _ := createUserVia(t, env, admin, "/admin/students", withNameFields(url.Values{"full_name": {"Иванов Пётр"}, "class": {strconv.FormatInt(class, 10)}}))
	anna, _ := createUserVia(t, env, admin, "/admin/students", servertest.NameForm("Иванова Анна"))
	createUserVia(t, env, admin, "/admin/students", servertest.NameForm("Петров Иван"))
	teacher, _ := createUserVia(t, env, admin, "/admin/teachers", servertest.NameForm("Сидорова Анна"))

	list := servertest.Get(t, env.Handler, "/admin/parents", admin).Body.String()
	assert.Contains(t, list, "нет детей")
	assert.NotContains(t, list, "Дети</h3>")

	edit := servertest.Get(t, env.Handler, parentsURL(parent, ""), admin).Body.String()
	assert.Contains(t, edit, "Дети</h3>")
	assert.Contains(t, edit, "Пока нет детей")
	assert.Contains(t, edit, `id="child-search" hx-preserve`)
	assert.Contains(t, edit, `name="edit" value="`+strconv.FormatInt(parent, 10)+`"`)
	assert.Contains(t, edit, `action="/admin/parents" hx-get="/admin/parents"`)
	assert.Contains(t, edit, `name="last_name" value="Иванова"`)
	assert.NotContains(t, edit, "Ничего не найдено")
	assert.Equal(t, 1, strings.Count(edit, "Дети</h3>"))

	const query = "иванов"

	search := servertest.Get(t, env.Handler, parentsURL(parent, query), admin).Body.String()
	assert.Contains(t, search, "Иванов Пётр")
	assert.Contains(t, search, "Иванова Анна")
	assert.Contains(t, search, ">7А</span>")
	assert.Contains(t, search, `name="student_id" value="`+strconv.FormatInt(petr, 10)+`"`)
	assert.Contains(t, search, `action="`+attr(childrenPath(parent, "", query))+`"`)
	assert.NotContains(t, search, "Петров Иван")
	assert.NotContains(t, search, "Сидорова Анна")

	nothing := servertest.Get(t, env.Handler, parentsURL(parent, "zzz"), admin).Body.String()
	assert.Contains(t, nothing, "Ничего не найдено")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, childrenPath(parent, "", query), studentForm(petr), []*http.Cookie{admin}, nil), parentsURL(parent, query))

	after := servertest.Get(t, env.Handler, parentsURL(parent, query), admin).Body.String()
	assert.Contains(t, after, `action="`+attr(childrenPath(parent, "/"+strconv.FormatInt(petr, 10)+"/remove", query))+`"`)
	assert.Contains(t, after, `name="student_id" value="`+strconv.FormatInt(anna, 10)+`"`)
	assert.NotContains(t, after, `name="student_id" value="`+strconv.FormatInt(petr, 10)+`"`)
	assert.NotContains(t, after, "Пока нет детей")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), studentForm(anna), []*http.Cookie{admin}, nil), parentsURL(parent, ""))
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), studentForm(anna), []*http.Cookie{admin}, nil), parentsURL(parent, ""))
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, childrenPath(other, "", ""), studentForm(petr), []*http.Cookie{admin}, nil), parentsURL(other, ""))

	children, err := env.School.Children(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int64{petr, anna}, children[parent])
	assert.Equal(t, []int64{petr}, children[other])

	list = servertest.Get(t, env.Handler, "/admin/parents", admin).Body.String()
	assert.Contains(t, list, "Иванов Пётр, Иванова Анна")
	assert.NotContains(t, list, "нет детей")

	for name, form := range map[string]url.Values{
		"teacher": studentForm(teacher),
		"garbage": {"student_id": {"abc"}},
		"unknown": studentForm(999),
	} {
		rejected := servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), form, []*http.Cookie{admin}, nil)
		assert.Equal(t, http.StatusOK, rejected.Code, name)
		assert.Contains(t, rejected.Body.String(), "<html", name)
		assert.Contains(t, rejected.Body.String(), "Такого ученика нет", name)
		assert.Contains(t, rejected.Body.String(), `name="last_name" value="Иванова"`, name)
	}

	remove := childrenPath(parent, "/"+strconv.FormatInt(petr, 10)+"/remove", "")
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, remove, nil, []*http.Cookie{admin}, nil), parentsURL(parent, ""))
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, remove, nil, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, childrenPath(parent, "/abc/remove", ""), nil, []*http.Cookie{admin}, nil).Code)

	children, err = env.School.Children(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int64{anna}, children[parent])
	assert.Equal(t, []int64{petr}, children[other])
}

func TestParentChildrenHtmxFragment(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	htmx := map[string]string{"HX-Request": "true"}

	parent, _ := createUserVia(t, env, admin, "/admin/parents", servertest.NameForm("Иванова Мария"))
	petr, _ := createUserVia(t, env, admin, "/admin/students", servertest.NameForm("Иванов Пётр"))

	added := servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), studentForm(petr), []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, added.Code)
	assert.NotContains(t, added.Body.String(), "<html")
	assert.Contains(t, added.Body.String(), `id="users"`)
	assert.Contains(t, added.Body.String(), "Дети</h3>")
	assert.Contains(t, added.Body.String(), "Иванов Пётр")

	rejected := servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), studentForm(999), []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, rejected.Code)
	assert.NotContains(t, rejected.Body.String(), "<html")
	assert.Contains(t, rejected.Body.String(), "Такого ученика нет")

	removed := servertest.PostForm(t, env.Handler, childrenPath(parent, "/"+strconv.FormatInt(petr, 10)+"/remove", ""), nil, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, removed.Code)
	assert.NotContains(t, removed.Body.String(), "<html")
	assert.Contains(t, removed.Body.String(), "Пока нет детей")
}

func TestParentChildrenRejectForeignSectionsAndRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.LoginAs(t, "teacher")

	parent, _ := createUserVia(t, env, admin, "/admin/parents", servertest.NameForm("Иванова Мария"))
	petr, _ := createUserVia(t, env, admin, "/admin/students", servertest.NameForm("Иванов Пётр"))

	for _, path := range []string{
		userPathFor("/admin/teachers", parent, "/children"),
		userPathFor("/admin/students", petr, "/children"),
		userPathFor("/admin/parents", petr, "/children"),
		userPathFor("/admin/parents", 999, "/children"),
		userPathFor("/admin/parents", petr, "/children/"+strconv.FormatInt(petr, 10)+"/remove"),
	} {
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, path, studentForm(petr), []*http.Cookie{admin}, nil).Code, path)
	}

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), studentForm(petr), nil, nil), "/login")
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, childrenPath(parent, "", ""), studentForm(petr), []*http.Cookie{teacher}, nil).Code)

	children, err := env.School.Children(t.Context())
	require.NoError(t, err)
	assert.Empty(t, children)
}
