package journal_test

import (
	"html"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func adminJournalsQuery(f fixture) string {
	return "/admin/journals?" + url.Values{
		"year":    {f.year()},
		"class":   {strconv.FormatInt(f.class, 10)},
		"subject": {strconv.FormatInt(f.subject, 10)},
	}.Encode()
}

func TestAdminJournals(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	admin := servertest.Login(t, f.env.Handler)

	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))

	substitute := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	_, err := f.env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substitute, StartDate: dateValue(f.today.AddDate(0, 0, -2)), EndDate: dateValue(f.today),
	})
	require.NoError(t, err)

	lesson := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	replaced := openLesson(t, f.env, f.env.LoginAs(t, "substitute"), f.class, f.subject, f.today.AddDate(0, 0, -1))
	require.NoError(t, f.env.Journal.UpdateTopic(ctx, f.teacher, lesson, "Дроби"))
	require.NoError(t, f.env.Journal.SaveHomework(ctx, f.teacher, lesson, journalHomework("Упр. 12")))
	f.env.SetCommentedMark(t, f.teacher, lesson, maria, firstWorkType(t, f), "5-", "Без пояснений")
	f.env.SetRecord(t, f.teacher, lesson, maria, true, "Опоздала")

	body := servertest.Get(t, f.env.Handler, "/admin/journals", admin).Body.String()
	assert.Contains(t, body, `href="/admin/journals" aria-current="page"`)
	assert.Contains(t, body, "Выберите класс и предмет")
	assert.Contains(t, body, `name="year"`)

	body = servertest.Get(t, f.env.Handler, adminJournalsQuery(f), admin).Body.String()
	assert.Contains(t, body, "Всего: 2 урока")
	assert.Contains(t, body, "Учитель: Сидорова А. А.")
	assert.Contains(t, body, `title="Урок провёл Кузнецов В. П."`)
	assert.Contains(t, body, `id="journal-modal"`)
	assert.Contains(t, body, `href="/admin/journals/lessons/`+strconv.FormatInt(lesson, 10)+`/students/`+strconv.FormatInt(maria, 10)+`"`)
	assert.Contains(t, body, `href="/admin/journals/lessons/`+strconv.FormatInt(lesson, 10)+`" title="Домашнее задание"`)
	assert.NotContains(t, body, `href="/admin/journals/lessons/`+strconv.FormatInt(replaced, 10)+`"`)
	assert.NotContains(t, body, "Дата нового урока")
	assert.NotContains(t, body, `name="topic"`)

	fragment := servertest.Get(t, f.env.Handler, adminJournalsQuery(f), admin, map[string]string{"HX-Request": "true", "HX-Target": "admin-journals"})
	assert.Contains(t, fragment.Body.String(), `id="admin-journals"`)
	assert.NotContains(t, fragment.Body.String(), "<html")

	window := servertest.Get(t, f.env.Handler, "/admin/journals/lessons/"+strconv.FormatInt(lesson, 10), admin, modalHeaders())
	assert.Equal(t, "false", window.Header().Get("HX-Push-Url"))
	card := window.Body.String()
	assert.Contains(t, card, "Провёл: Сидорова А. А.")
	assert.Contains(t, card, "Тема: Дроби")
	assert.Contains(t, card, "Упр. 12")
	assert.NotContains(t, card, "<form")

	entryPath := "/admin/journals/lessons/" + strconv.FormatInt(lesson, 10) + "/students/" + strconv.FormatInt(maria, 10)
	entry := servertest.Get(t, f.env.Handler, entryPath, admin, modalHeaders()).Body.String()
	assert.Contains(t, entry, "Иванова Мария · ")
	assert.Contains(t, entry, ">Отсутствовал<")
	assert.Contains(t, entry, "Опоздала")
	assert.Contains(t, entry, ">5−<")
	assert.Contains(t, entry, "Без пояснений")
	assert.Contains(t, entry, `href="`+html.EscapeString("/admin/diary/"+strconv.FormatInt(maria, 10)+"?"+url.Values{"date": {dateValue(f.today)}, "lesson": {strconv.FormatInt(lesson, 10)}}.Encode())+`"`)
	assert.NotContains(t, entry, "<form")

	page := servertest.Get(t, f.env.Handler, entryPath, admin).Body.String()
	assert.Contains(t, page, "← К журналу")

	for _, cookie := range []*http.Cookie{teacher, f.env.LoginAs(t, "maria")} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, adminJournalsQuery(f), cookie).Code)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, entryPath, cookie).Code)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journals/lessons/"+strconv.FormatInt(lesson, 10), cookie).Code)
	}

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journals/lessons/9999", admin).Code)
	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, adminJournalsQuery(f)), "/login")
}
