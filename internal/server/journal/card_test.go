package journal_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/view"
)

func modalHeaders() map[string]string {
	return map[string]string{"HX-Request": "true", "HX-Target": "modal-body"}
}

type cardField struct {
	name  string
	value string
}

type cardFile struct {
	name    string
	content string
}

func smallFile(name string) cardFile {
	return cardFile{name: name, content: "content of " + name}
}

func saveCard(t *testing.T, handler http.Handler, lessonID int64, cookie *http.Cookie, headers map[string]string, fields []cardField, files ...cardFile) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, field := range fields {
		require.NoError(t, writer.WriteField(field.name, field.value))
	}

	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.name)
		require.NoError(t, err)
		_, err = io.WriteString(part, file.content)
		require.NoError(t, err)
	}

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, lessonPath(lessonID, ""), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func TestLessonCard(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	lesson := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	id := strconv.FormatInt(lesson, 10)

	page := servertest.Get(t, f.env.Handler, lessonPath(lesson, ""), teacher).Body.String()
	assert.Contains(t, page, "Урок "+view.FormatShortDate(f.today))
	assert.Contains(t, page, "← К журналу")
	assert.Contains(t, page, `href="`+pairPath(f.class, f.subject)+"#lesson-"+id+`"`)
	assert.Contains(t, page, `id="card-homework-text"`)
	assert.Contains(t, page, `name="due"`)
	assert.Contains(t, page, `name="topic"`)
	assert.NotContains(t, page, `name="date"`)
	assert.Contains(t, page, `form="lesson-card-form"`)
	assert.NotContains(t, page, "Удалить урок")

	window := servertest.Get(t, f.env.Handler, lessonPath(lesson, ""), teacher, modalHeaders())
	assert.Equal(t, "false", window.Header().Get("HX-Push-Url"))
	assert.Contains(t, window.Body.String(), `id="journal-modal-title"`)
	assert.Contains(t, window.Body.String(), "data-modal-close")
	assert.NotContains(t, window.Body.String(), "<html")

	due := f.today.AddDate(0, 0, 2)
	saved := saveCard(t, f.env.Handler, lesson, teacher, modalHeaders(),
		[]cardField{{"topic", "Дроби"}, {"text", "Упр. 12"}, {"due", inputDate(due)}}, smallFile("a.txt"))
	require.Equal(t, http.StatusOK, saved.Code)
	assert.Equal(t, "journal:close-modal", saved.Header().Get("HX-Trigger"))
	assert.Contains(t, saved.Body.String(), `id="journal-content" hx-swap-oob="true"`)
	assert.Contains(t, saved.Body.String(), "data-highlight")
	assert.Contains(t, saved.Body.String(), "bg-emerald-50")
	assert.Contains(t, saved.Body.String(), `value="Дроби"`)

	homework, err := f.env.Journal.Homework(ctx, lesson)
	require.NoError(t, err)
	assert.Equal(t, "Упр. 12", homework.Text)
	assert.True(t, due.Equal(homework.Due))
	require.Len(t, homework.Files, 1)

	invalid := saveCard(t, f.env.Handler, lesson, teacher, modalHeaders(),
		[]cardField{{"topic", strings.Repeat("а", 201)}, {"text", "Другое"}, {"due", "вчера"}}, smallFile("b.txt"))
	require.Equal(t, http.StatusOK, invalid.Code)
	assert.Empty(t, invalid.Header().Get("HX-Trigger"))
	assert.Contains(t, invalid.Body.String(), "Неверная дата")
	assert.Contains(t, invalid.Body.String(), "Тема не длиннее 200 символов")
	assert.Contains(t, invalid.Body.String(), ">Другое</textarea>")

	tooMany := saveCard(t, f.env.Handler, lesson, teacher, modalHeaders(),
		[]cardField{{"text", "Другое"}}, smallFile("c.txt"), smallFile("d.txt"), smallFile("e.txt"))
	assert.Contains(t, tooMany.Body.String(), "Не больше 3 файлов у одного урока")

	tooLarge := saveCard(t, f.env.Handler, lesson, teacher, modalHeaders(),
		[]cardField{{"text", "Другое"}}, cardFile{name: "big.txt", content: strings.Repeat("а", 1<<20)})
	assert.Contains(t, tooLarge.Body.String(), "Файл больше 1 МБ")

	homework, err = f.env.Journal.Homework(ctx, lesson)
	require.NoError(t, err)
	assert.Equal(t, "Упр. 12", homework.Text)
	assert.Len(t, homework.Files, 1)

	cleared := saveCard(t, f.env.Handler, lesson, teacher, modalHeaders(),
		[]cardField{{"topic", "Дроби"}, {"text", ""}, {"due", ""}, {"remove", homework.Files[0].ID}})
	require.Equal(t, http.StatusOK, cleared.Code)
	assert.NotContains(t, cleared.Body.String(), "bg-emerald-50")

	homework, err = f.env.Journal.Homework(ctx, lesson)
	require.NoError(t, err)
	assert.True(t, homework.Empty())

	plain := saveCard(t, f.env.Handler, lesson, teacher, nil, []cardField{{"topic", "Дроби"}, {"text", "Упр. 13"}})
	servertest.AssertRedirect(t, plain, pairPath(f.class, f.subject)+"#lesson-"+id)
}

func TestLessonCardForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))
	lesson := openLesson(t, f.env, f.env.LoginAs(t, "teacher"), f.class, f.subject, f.today)
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	createUser(t, f.env, auth.RoleStudent, "student", "Иванова Мария Петровна")

	for _, cookie := range []*http.Cookie{f.env.LoginAs(t, "stranger"), f.env.LoginAs(t, "student"), servertest.Login(t, f.env.Handler)} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, lessonPath(lesson, ""), cookie).Code)
		assert.Equal(t, http.StatusNotFound, saveCard(t, f.env.Handler, lesson, cookie, modalHeaders(), []cardField{{"text", "Чужое"}}).Code)
	}
}
