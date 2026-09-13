package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/auth"
)

func TestRoleSectionsAreHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.createUser(t, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	env.createUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")

	student := env.loginAs(t, "student")
	teacher := env.loginAs(t, "teacher")
	admin := login(t, env.handler)

	assertRedirect(t, get(t, env.handler, "/journal"), "/login")
	assertRedirect(t, get(t, env.handler, "/diary"), "/login")

	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/journal", student).Code)
	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/journal", admin).Code)
	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/diary", teacher).Code)
	assert.Equal(t, http.StatusNotFound, get(t, env.handler, "/diary", admin).Code)

	diary := get(t, env.handler, "/diary", student)
	assert.Equal(t, http.StatusOK, diary.Code)
	assert.Contains(t, diary.Body.String(), "Раздел в разработке")
	assert.Contains(t, diary.Body.String(), `href="/diary"`)
	assert.NotContains(t, diary.Body.String(), `href="/admin/classes"`)

	journal := get(t, env.handler, "/journal", teacher)
	assert.Equal(t, http.StatusOK, journal.Code)
	assert.Contains(t, journal.Body.String(), `href="/journal"`)
	assert.NotContains(t, journal.Body.String(), `href="/diary"`)
}
