package diary_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

func TestDiaryShowsHomework(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	parent := createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")
	require.NoError(t, f.env.School.AddChild(ctx, parent, f.student))

	algebra := f.openLesson(t, f.class, f.algebra, f.today)
	history := f.openLesson(t, f.class, f.history, f.today)
	due := f.today.AddDate(0, 0, 3)

	require.NoError(t, f.env.Journal.SaveHomework(ctx, f.teacher, algebra, journal.HomeworkInput{Text: "Упр. 12\nстр. 40", Due: due.Format(validation.DateLayout)}))
	require.NoError(t, f.env.Journal.AddHomeworkFile(ctx, f.teacher, algebra, "задание.txt", strings.NewReader("Упр. 12")))

	homework, err := f.env.Journal.Homework(ctx, algebra)
	require.NoError(t, err)
	require.Len(t, homework.Files, 1)
	fileHref := `href="/files/` + homework.Files[0].ID + `"`

	for _, login := range []string{"student", "parent"} {
		code, body := get(t, f.env, "/diary", f.env.LoginAs(t, login), false)
		require.Equal(t, http.StatusOK, code, login)
		assert.Contains(t, body, ">ДЗ<", login)
		assert.Contains(t, body, ">Домашнее задание<", login)
		assert.Contains(t, body, ">к "+due.Format("02.01.2006")+"<", login)
		assert.Contains(t, body, ">Упр. 12\nстр. 40</p>", login)
		assert.Contains(t, body, fileHref, login)
		assert.Contains(t, body, ">задание.txt</a>", login)
	}

	admin := servertest.Login(t, f.env.Handler)
	code, body := get(t, f.env, "/admin/diary/"+idValue(f.student), admin, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, fileHref)

	code, body = get(t, f.env, diaryURL("/diary", f.today, url.Values{"lesson": {idValue(history)}}), f.env.LoginAs(t, "student"), false)
	require.Equal(t, http.StatusOK, code)
	assert.NotContains(t, body, ">Домашнее задание<")
	assert.Contains(t, body, ">ДЗ<")
}
