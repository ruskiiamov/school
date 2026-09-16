package diary_test

import (
	"html"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
)

func TestParentDiaryChildrenSwitch(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	parent := createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")
	second := createUser(t, f.env, auth.RoleStudent, "second", "Иванов Артём Петрович")
	gone := createUser(t, f.env, auth.RoleStudent, "gone", "Иванов Ушедший Петрович")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.other, second))
	require.NoError(t, f.env.School.AddChild(ctx, parent, f.student))
	require.NoError(t, f.env.School.AddChild(ctx, parent, second))
	require.NoError(t, f.env.School.AddChild(ctx, parent, gone))
	require.NoError(t, f.env.Auth.SetUserActive(ctx, gone, false))

	algebra := f.openLesson(t, f.class, f.algebra, f.today)
	otherLesson := f.openLesson(t, f.other, f.algebra, f.today)
	_, err := f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 5})
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, otherLesson, second, journal.MarkInput{WorkTypeID: f.workType, Value: 3})
	require.NoError(t, err)

	cookie := f.env.LoginAs(t, "parent")
	secondQuery := url.Values{"child": {idValue(second)}}
	firstQuery := url.Values{"child": {idValue(f.student)}}

	code, body := get(t, f.env, "/diary", cookie, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today, secondQuery))+`" hx-get`)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today, firstQuery))+`" hx-get`)
	assert.Less(t, indexOf(body, "Иванов Артём Петрович"), indexOf(body, "Иванова Мария Петровна"))
	assert.NotContains(t, body, "Иванов Ушедший Петрович")
	assert.Contains(t, body, ">3<")
	assert.NotContains(t, body, ">5<")
	assert.Contains(t, body, `name="child" value="`+idValue(second)+`"`)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today.AddDate(0, 0, -1), secondQuery))+`"`)

	code, body = get(t, f.env, diaryURL("/diary", f.today, firstQuery), cookie, true)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="diary"`)
	assert.Contains(t, body, ">5<")
	assert.NotContains(t, body, ">3<")
	assert.NotContains(t, body, "<html")

	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(f.stranger)}}), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(gone)}}), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(parent)}}), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(f.student)}, "lesson": {idValue(otherLesson)}}), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestParentDiaryWithoutChildren(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")
	cookie := f.env.LoginAs(t, "parent")

	code, body := get(t, f.env, "/diary", cookie, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "К вашему аккаунту не привязаны дети")
	assert.NotContains(t, body, `name="date"`)

	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(f.student)}}), cookie, false)
	assert.Equal(t, http.StatusNotFound, code)
}
