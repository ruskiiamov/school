package journal_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

var fileHref = regexp.MustCompile(`href="/files/([0-9a-f]{32})"`)

func upload(t *testing.T, handler http.Handler, path string, cookie *http.Cookie, htmx bool, files map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for name, content := range files {
		part, err := writer.CreateFormFile("files", name)
		require.NoError(t, err)
		_, err = io.WriteString(part, content)
		require.NoError(t, err)
	}

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	if htmx {
		req.Header.Set("HX-Request", "true")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func TestLessonHomeworkTextAndDue(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	homeworkPath := lessonPath(id, "/homework")

	body := servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	assert.Contains(t, body, `<details id="lesson-homework" class=`)
	assert.Contains(t, body, ">не задано<")
	assert.Contains(t, body, `action="`+homeworkPath+`"`)
	assert.Contains(t, body, `action="`+homeworkPath+`/files"`)
	assert.Contains(t, body, `id="lesson-homework-text" hx-preserve`)

	saved := servertest.PostForm(t, f.env.Handler, homeworkPath,
		url.Values{"text": {"Упр. 12,  13\r\n\r\n\r\n  стр. 40 "}, "due": {dateValue(f.today.AddDate(0, 0, 2))}},
		[]*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, saved, lessonPath(id, ""))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	assert.Contains(t, body, ">Упр. 12, 13\n\nстр. 40</textarea>")
	assert.Contains(t, body, `name="due" value="`+dateValue(f.today.AddDate(0, 0, 2))+`"`)
	assert.Contains(t, body, ">к "+f.today.AddDate(0, 0, 2).Format("02.01.2006")+"<")
	assert.Contains(t, body, "Удалить урок")

	fragment := servertest.PostForm(t, f.env.Handler, homeworkPath, url.Values{"text": {"Повторить"}, "due": {""}},
		[]*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `<details id="lesson-homework" open class=`)
	assert.Contains(t, fragment.Body.String(), ">задано<")
	assert.NotContains(t, fragment.Body.String(), "<html")

	long := servertest.PostForm(t, f.env.Handler, homeworkPath, url.Values{"text": {strings.Repeat("а", 2001)}}, []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, long.Code)
	assert.Contains(t, long.Body.String(), "Задание не длиннее 2000 символов")
	assert.Contains(t, long.Body.String(), `<details id="lesson-homework" open class=`)

	badDate := servertest.PostForm(t, f.env.Handler, homeworkPath, url.Values{"text": {"x"}, "due": {"2026-13-40"}}, []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, badDate.Code)
	assert.Contains(t, badDate.Body.String(), "Неверная дата")
	assert.Contains(t, badDate.Body.String(), `name="due" value="2026-13-40"`)

	cleared := servertest.PostForm(t, f.env.Handler, homeworkPath, url.Values{"text": {" "}, "due": {""}}, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, cleared, lessonPath(id, ""))

	var rows int
	require.NoError(t, f.env.DB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM homework").Scan(&rows))
	assert.Equal(t, 0, rows)

	body = servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	assert.Contains(t, body, ">не задано<")
}

func TestLessonHomeworkFiles(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	uploadPath := lessonPath(id, "/homework/files")

	empty := upload(t, f.env.Handler, uploadPath, teacher, false, nil)
	assert.Equal(t, http.StatusOK, empty.Code)
	assert.Contains(t, empty.Body.String(), "Выберите файл")

	uploaded := upload(t, f.env.Handler, uploadPath, teacher, false, map[string]string{`C:\docs\Задание №1.txt`: "Упражнение 12"})
	servertest.AssertRedirect(t, uploaded, lessonPath(id, ""))

	body := servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	assert.Contains(t, body, ">Задание №1.txt</a>")
	assert.Contains(t, body, ">1 файл<")
	assert.Contains(t, body, ">23 Б<")

	match := fileHref.FindStringSubmatch(body)
	require.NotNil(t, match)
	fileID := match[1]

	ids, err := f.env.Files.IDs()
	require.NoError(t, err)
	assert.Equal(t, []string{fileID}, ids)

	download := servertest.Get(t, f.env.Handler, "/files/"+fileID, teacher)
	require.Equal(t, http.StatusOK, download.Code)
	assert.Equal(t, "Упражнение 12", download.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", download.Header().Get("Content-Type"))
	assert.Contains(t, download.Header().Get("Content-Disposition"), "attachment; filename*=utf-8''")

	tooLarge := upload(t, f.env.Handler, uploadPath, teacher, true, map[string]string{"big.bin": strings.Repeat("x", 1<<20+1)})
	assert.Equal(t, http.StatusOK, tooLarge.Code)
	assert.Contains(t, tooLarge.Body.String(), "Файл больше 1 МБ")
	assert.Contains(t, tooLarge.Body.String(), `<details id="lesson-homework" open class=`)

	ids, err = f.env.Files.IDs()
	require.NoError(t, err)
	assert.Len(t, ids, 1)

	more := upload(t, f.env.Handler, uploadPath, teacher, true, map[string]string{"a.txt": "a", "b.txt": "b"})
	assert.Equal(t, http.StatusOK, more.Code)
	assert.Contains(t, more.Body.String(), ">3 файла<")
	assert.NotContains(t, more.Body.String(), `action="`+uploadPath+`"`)

	overflow := upload(t, f.env.Handler, uploadPath, teacher, false, map[string]string{"d.txt": "d"})
	assert.Equal(t, http.StatusOK, overflow.Code)
	assert.Contains(t, overflow.Body.String(), "Не больше 3 файлов у одного урока")

	deleted := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/homework/files/"+fileID+"/delete"), nil, []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, deleted.Code)
	assert.NotContains(t, deleted.Body.String(), "Задание №1.txt")
	assert.Contains(t, deleted.Body.String(), ">2 файла<")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/files/"+fileID, teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/homework/files/"+fileID+"/delete"), nil, []*http.Cookie{teacher}, nil).Code)

	ids, err = f.env.Files.IDs()
	require.NoError(t, err)
	assert.Len(t, ids, 2)

	removed := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/delete"), nil, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, removed, "/journal")

	ids, err = f.env.Files.IDs()
	require.NoError(t, err)
	assert.Empty(t, ids)

	var rows int
	require.NoError(t, f.env.DB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM homework_files").Scan(&rows))
	assert.Equal(t, 0, rows)
}

func TestLessonHomeworkForeignTeacher(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Чужой Учитель Иванович")
	stranger := f.env.LoginAs(t, "stranger")

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	uploaded := upload(t, f.env.Handler, lessonPath(id, "/homework/files"), teacher, false, map[string]string{"a.txt": "a"})
	servertest.AssertRedirect(t, uploaded, lessonPath(id, ""))

	body := servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	fileID := fileHref.FindStringSubmatch(body)[1]

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/homework"), url.Values{"text": {"x"}}, []*http.Cookie{stranger}, nil).Code)
	assert.Equal(t, http.StatusNotFound, upload(t, f.env.Handler, lessonPath(id, "/homework/files"), stranger, false, map[string]string{"b.txt": "b"}).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/homework/files/"+fileID+"/delete"), nil, []*http.Cookie{stranger}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/files/"+fileID, stranger).Code)

	other := openLesson(t, f.env, teacher, f.class, f.subject, f.today.AddDate(0, 0, 1))
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(other, "/homework/files/"+fileID+"/delete"), nil, []*http.Cookie{teacher}, nil).Code)
}

func TestJournalCleanupRemovesHomeworkOrphans(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	uploaded := upload(t, f.env.Handler, lessonPath(id, "/homework/files"), teacher, false, map[string]string{"a.txt": "a"})
	servertest.AssertRedirect(t, uploaded, lessonPath(id, ""))
	require.NoError(t, f.env.Journal.SaveHomework(t.Context(), f.teacher, id, journalHomework("Упр. 1")))

	_, err := f.env.DB.ExecContext(t.Context(), "DELETE FROM lessons WHERE id = ?", id)
	require.NoError(t, err)

	f.env.Journal.CleanupOrphans(t.Context())

	var homework, files int
	require.NoError(t, f.env.DB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM homework").Scan(&homework))
	require.NoError(t, f.env.DB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM homework_files").Scan(&files))
	assert.Equal(t, 0, homework)
	assert.Equal(t, 0, files)

	ids, err := f.env.Files.IDs()
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func saveStray(t *testing.T, env *servertest.Env, age time.Duration) string {
	t.Helper()

	stray, err := env.Files.Save(strings.NewReader("stray"), 100)
	require.NoError(t, err)

	modified := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(filepath.Join(env.FilesDir, stray.ID), modified, modified))

	return stray.ID
}

func TestJournalCleanupRemovesOnlyOldStrayFiles(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	uploaded := upload(t, f.env.Handler, lessonPath(id, "/homework/files"), teacher, false, map[string]string{"a.txt": "a"})
	servertest.AssertRedirect(t, uploaded, lessonPath(id, ""))

	fresh := saveStray(t, f.env, time.Hour)
	old := saveStray(t, f.env, 25*time.Hour)

	f.env.Journal.CleanupOrphans(t.Context())

	ids, err := f.env.Files.IDs()
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, fresh)
	assert.NotContains(t, ids, old)
}

func TestJournalCleanupKeepsFilesWhenDatabaseKnowsNone(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	old := saveStray(t, f.env, 25*time.Hour)

	f.env.Journal.CleanupOrphans(t.Context())

	ids, err := f.env.Files.IDs()
	require.NoError(t, err)
	assert.Equal(t, []string{old}, ids)
}
