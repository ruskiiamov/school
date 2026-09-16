package journal_test

import (
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestAdminJournalListAndLesson(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	admin := servertest.Login(t, f.env.Handler)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, petr))

	workTypes, err := f.env.School.WorkTypes(ctx, false)
	require.NoError(t, err)

	pair := url.Values{"class": {strconv.FormatInt(f.class, 10)}, "subject": {strconv.FormatInt(f.subject, 10)}}
	listPath := "/admin/journal?" + pair.Encode()

	body := servertest.Get(t, f.env.Handler, "/admin/journal", admin).Body.String()
	assert.Contains(t, body, "Выберите класс и предмет")
	assert.Contains(t, body, `href="/admin/journal" aria-current="page"`)
	assert.Contains(t, body, ">7А</option>")
	assert.Contains(t, body, ">Алгебра</option>")

	body = servertest.Get(t, f.env.Handler, listPath, admin).Body.String()
	assert.Contains(t, body, "Уроков пока нет")

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	older := openLesson(t, f.env, teacher, f.class, f.subject, f.today.AddDate(0, 0, -1))
	require.NoError(t, f.env.Journal.UpdateTopic(ctx, f.teacher, id, "Квадратные уравнения"))
	_, err = f.env.Journal.AddMark(ctx, f.teacher, id, maria, journal.MarkInput{WorkTypeID: workTypes[0].ID, Value: 5, Label: "у доски"})
	require.NoError(t, err)
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, id, petr, journal.RecordInput{Absent: true, Comment: "Опоздал"}))
	require.NoError(t, f.env.Journal.SaveHomework(ctx, f.teacher, id, journal.HomeworkInput{Text: "Упр. 12", Due: dateValue(f.today.AddDate(0, 0, 2))}))
	require.NoError(t, f.env.Journal.AddHomeworkFile(ctx, f.teacher, id, "задание.txt", strings.NewReader("Упр. 12")))

	fragment := servertest.Get(t, f.env.Handler, listPath, admin, map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, fragment.Code)
	body = fragment.Body.String()
	assert.Contains(t, body, `id="admin-journal"`)
	assert.NotContains(t, body, "<html")
	assert.Contains(t, body, `href="/admin/journal/lessons/`+strconv.FormatInt(id, 10)+`"`)
	assert.Contains(t, body, `href="/admin/journal/lessons/`+strconv.FormatInt(older, 10)+`"`)
	assert.Less(t, strings.Index(body, "/lessons/"+strconv.FormatInt(id, 10)+`"`), strings.Index(body, "/lessons/"+strconv.FormatInt(older, 10)+`"`))
	assert.Contains(t, body, "Сидорова А.")
	assert.Contains(t, body, "Квадратные уравнения")
	assert.Equal(t, 1, strings.Count(body, ">ДЗ<"))
	assert.Contains(t, body, `<option value="`+strconv.FormatInt(f.class, 10)+`" selected>7А</option>`)

	lessonPath := "/admin/journal/lessons/" + strconv.FormatInt(id, 10)
	page := servertest.Get(t, f.env.Handler, lessonPath, admin)
	require.Equal(t, http.StatusOK, page.Code)
	body = page.Body.String()
	assert.Contains(t, body, "7А · Алгебра · "+f.today.Format("02.01.2006"))
	assert.Contains(t, body, `href="`+html.EscapeString(listPath)+`"`)
	assert.Contains(t, body, "К списку")
	assert.Contains(t, body, "Сидорова Анна Андреевна")
	assert.Contains(t, body, "Тема: Квадратные уравнения")
	assert.Contains(t, body, ">Домашнее задание<")
	assert.Contains(t, body, ">задание.txt</a>")
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, ">5<")
	assert.Contains(t, body, "у доски")
	assert.Contains(t, body, ">Н<")
	assert.Contains(t, body, `href="`+html.EscapeString("/admin/diary/"+strconv.FormatInt(maria, 10)+"?date="+dateValue(f.today)+"&lesson="+strconv.FormatInt(id, 10))+`"`)
	assert.NotContains(t, body, `hx-post="/journal/`)
	assert.NotContains(t, body, "Поставить")
	assert.NotContains(t, body, `id="lesson-homework"`)

	selected := servertest.Get(t, f.env.Handler, lessonPath+"?student="+strconv.FormatInt(petr, 10), admin, map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, selected.Code)
	body = selected.Body.String()
	assert.Contains(t, body, `id="lesson"`)
	assert.NotContains(t, body, "<html")
	assert.Contains(t, body, ">Отсутствовал<")
	assert.Contains(t, body, "Опоздал")
	assert.Contains(t, body, "Оценок нет")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journal/lessons/999", admin).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journal/lessons/abc", admin).Code)
}

func TestAdminJournalHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))
	createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	createUser(t, f.env, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	for _, login := range []string{"teacher", "maria", "parent"} {
		cookie := f.env.LoginAs(t, login)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journal", cookie).Code, login)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/journal/lessons/"+strconv.FormatInt(id, 10), cookie).Code, login)
	}
}
