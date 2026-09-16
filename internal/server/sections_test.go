package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestRoleSectionsAreHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")

	student := env.LoginAs(t, "student")
	teacher := env.LoginAs(t, "teacher")
	admin := servertest.Login(t, env.Handler)

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/journal"), "/login")
	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/diary"), "/login")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/journal", student).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/journal", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/diary", teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/diary", admin).Code)

	diary := servertest.Get(t, env.Handler, "/diary", student)
	assert.Equal(t, http.StatusOK, diary.Code)
	assert.Contains(t, diary.Body.String(), `href="/diary"`)
	assert.NotContains(t, diary.Body.String(), `href="/admin/classes"`)

	journal := servertest.Get(t, env.Handler, "/journal", teacher)
	assert.Equal(t, http.StatusOK, journal.Code)
	assert.Contains(t, journal.Body.String(), `href="/journal"`)
	assert.NotContains(t, journal.Body.String(), `href="/diary"`)
}
