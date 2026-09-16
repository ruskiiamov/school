package diary_test

import (
	"html"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/view"
)

func TestStudentDiaryDayAndLesson(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	student := f.env.LoginAs(t, "student")

	code, body := get(t, f.env, "/diary", student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Уроков в этот день нет")
	assert.Contains(t, body, view.FormatDate(f.today))
	assert.Contains(t, body, `name="date" value="`+f.today.Format("2006-01-02")+`"`)
	assert.Contains(t, body, `href="`+diaryURL("/diary", f.today.AddDate(0, 0, -1), nil)+`"`)
	assert.Contains(t, body, `href="`+diaryURL("/diary", f.today.AddDate(0, 0, 1), nil)+`"`)

	history := f.openLesson(t, f.class, f.history, f.today)
	algebra := f.openLesson(t, f.class, f.algebra, f.today)
	yesterday := f.openLesson(t, f.class, f.algebra, f.today.AddDate(0, 0, -1))
	foreign := f.openLesson(t, f.other, f.algebra, f.today)

	require.NoError(t, f.env.Journal.UpdateTopic(ctx, f.teacher, algebra, "Квадратные уравнения"))
	_, err := f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 5, Label: "у доски"})
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, algebra, f.student, journal.MarkInput{WorkTypeID: f.workType, Value: 4})
	require.NoError(t, err)
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, algebra, f.student, journal.RecordInput{Absent: true, Comment: "Опоздала на 10 минут"}))
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, history, f.student, journal.RecordInput{Absent: true}))
	_, err = f.env.Journal.AddMark(ctx, f.teacher, foreign, f.stranger, journal.MarkInput{WorkTypeID: f.workType, Value: 3})
	require.NoError(t, err)

	code, body = get(t, f.env, "/diary", student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today, url.Values{"lesson": {idValue(algebra)}}))+`" hx-get`)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today, url.Values{"lesson": {idValue(history)}}))+`" hx-get`)
	assert.Less(t, indexOf(body, ">Алгебра<"), indexOf(body, ">История<"))
	assert.Contains(t, body, "Сидорова А.")
	assert.Contains(t, body, ">5, 4 · Н<")
	assert.Contains(t, body, ">Н<")
	assert.Contains(t, body, "Алгебра · "+f.today.Format("02.01.2006"))
	assert.Contains(t, body, "Сидорова Анна Андреевна")
	assert.Contains(t, body, "Тема: Квадратные уравнения")
	assert.Contains(t, body, "у доски")
	assert.Contains(t, body, ">Отсутствовал<")
	assert.Contains(t, body, "Опоздала на 10 минут")
	assert.NotContains(t, body, "Уроков в этот день нет")
	assert.NotContains(t, body, "9Б")

	code, body = get(t, f.env, diaryURL("/diary", f.today, url.Values{"lesson": {idValue(history)}}), student, true)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="diary"`)
	assert.Contains(t, body, "История · "+f.today.Format("02.01.2006"))
	assert.Contains(t, body, "Оценок нет")
	assert.NotContains(t, body, "Опоздала")
	assert.NotContains(t, body, "<html")

	code, body = get(t, f.env, diaryURL("/diary", f.today.AddDate(0, 0, -1), nil), student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `href="`+html.EscapeString(diaryURL("/diary", f.today.AddDate(0, 0, -1), url.Values{"lesson": {idValue(yesterday)}}))+`" hx-get`)
	assert.NotContains(t, body, "История")

	code, _ = get(t, f.env, "/diary?date=garbage", student, false)
	assert.Equal(t, http.StatusOK, code)

	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"lesson": {idValue(foreign)}}), student, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"lesson": {idValue(yesterday)}}), student, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(f.stranger)}}), student, false)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, f.env, diaryURL("/diary", f.today, url.Values{"child": {idValue(f.student)}}), student, false)
	assert.Equal(t, http.StatusOK, code)

	require.NoError(t, f.env.School.RemoveClassStudent(ctx, f.class, f.student))

	code, body = get(t, f.env, "/diary", student, false)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, ">5, 4 · Н<")
	assert.Contains(t, body, ">История<")
}

func TestDiaryHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	admin := servertest.Login(t, f.env.Handler)

	for _, cookie := range []*http.Cookie{teacher, admin} {
		code, _ := get(t, f.env, "/diary", cookie, false)
		assert.Equal(t, http.StatusNotFound, code)
	}

	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, "/diary"), "/login")
}

func indexOf(body, needle string) int {
	for i := 0; i+len(needle) <= len(body); i++ {
		if body[i:i+len(needle)] == needle {
			return i
		}
	}

	return -1
}
