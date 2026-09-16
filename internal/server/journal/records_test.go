package journal_test

import (
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

func recordPath(lessonID, studentID int64) string {
	return lessonPath(lessonID, "/students/"+strconv.FormatInt(studentID, 10)) + studentQuery(studentID)
}

func TestLessonRecordAbsenceAndComment(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")
	outsider := createUser(t, f.env, auth.RoleStudent, "outsider", "Чужой Ученик")
	require.NoError(t, f.env.School.AddClassStudent(t.Context(), f.class, maria))
	require.NoError(t, f.env.School.AddClassStudent(t.Context(), f.class, petr))

	workTypes, err := f.env.School.WorkTypes(t.Context(), false)
	require.NoError(t, err)

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	body := servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria), teacher).Body.String()
	assert.Contains(t, body, `action="`+recordPath(id, maria)+`"`)
	assert.Contains(t, body, `name="absent" value="1"`)
	assert.Contains(t, body, `aria-pressed="false"`)
	assert.Contains(t, body, "Добавить комментарий")
	assert.NotContains(t, body, "<details open")

	toggled := servertest.PostForm(t, f.env.Handler, recordPath(id, maria), url.Values{"absent": {"1"}, "comment": {""}}, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, toggled, lessonPath(id, "")+studentQuery(maria))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria), teacher).Body.String()
	assert.Contains(t, body, `aria-pressed="true"`)
	assert.Contains(t, body, `name="absent" value="0"`)
	assert.Contains(t, body, ">Н<")

	saved := servertest.PostForm(t, f.env.Handler, recordPath(id, maria), url.Values{"absent": {"1"}, "comment": {" Болела,  справка "}}, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, saved, lessonPath(id, "")+studentQuery(maria))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria), teacher).Body.String()
	assert.Contains(t, body, `aria-pressed="true"`)
	assert.Contains(t, body, `name="comment" value="Болела, справка"`)
	assert.Contains(t, body, ">Болела, справка</p>")
	assert.Contains(t, body, ">Болела, справка</textarea>")
	assert.Contains(t, body, "Изменить комментарий")
	assert.Contains(t, body, ">Н<")
	assert.NotContains(t, body, `action="`+lessonPath(id, "/delete")+`"`)

	_, err = f.env.Journal.AddMark(t.Context(), f.teacher, id, maria, journalMarkInput(workTypes[0].ID, 5))
	require.NoError(t, err)

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria), teacher).Body.String()
	assert.Contains(t, body, ">5 · Н<")

	notEmpty := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/delete"), nil, []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, notEmpty.Code)
	assert.Contains(t, notEmpty.Body.String(), "Урок с оценками или записями удалить нельзя")

	long := servertest.PostForm(t, f.env.Handler, recordPath(id, maria), url.Values{"comment": {strings.Repeat("а", 501)}}, []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, long.Code)
	assert.Contains(t, long.Body.String(), "Комментарий не длиннее 500 символов")
	assert.Contains(t, long.Body.String(), "<details open")

	notMember := servertest.PostForm(t, f.env.Handler, recordPath(id, outsider), url.Values{"absent": {"1"}}, []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, notMember.Code)
	assert.Contains(t, notMember.Body.String(), "Ученик не в классе")
	assert.Contains(t, notMember.Body.String(), `id="lesson"`)
	assert.NotContains(t, notMember.Body.String(), "<html")

	cleared := servertest.PostForm(t, f.env.Handler, recordPath(id, maria), url.Values{"absent": {"0"}, "comment": {""}}, []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, cleared.Code)
	assert.Contains(t, cleared.Body.String(), `aria-pressed="false"`)
	assert.Contains(t, cleared.Body.String(), "Добавить комментарий")
	assert.Contains(t, cleared.Body.String(), ">5<")

	entries, err := f.env.Journal.LessonStudents(t.Context(), journal.Lesson{ID: id, ClassID: f.class})
	require.NoError(t, err)

	for _, entry := range entries {
		assert.False(t, entry.Absent)
		assert.Empty(t, entry.Comment)
	}

	require.NoError(t, f.env.Journal.SaveRecord(t.Context(), f.teacher, id, petr, journal.RecordInput{Absent: true}))
	require.NoError(t, f.env.School.RemoveClassStudent(t.Context(), f.class, petr))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(petr), teacher).Body.String()
	assert.Contains(t, body, "Козлов Пётр Ильич")
	assert.Contains(t, body, "не в классе")
	assert.NotContains(t, body, `action="`+recordPath(id, petr)+`"`)

	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	stranger := f.env.LoginAs(t, "stranger")
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, recordPath(id, maria), url.Values{"absent": {"1"}}, []*http.Cookie{stranger}, nil).Code)
}

func TestJournalCleanupRemovesOrphans(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(t.Context(), f.class, maria))

	workTypes, err := f.env.School.WorkTypes(t.Context(), false)
	require.NoError(t, err)

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	_, err = f.env.Journal.AddMark(t.Context(), f.teacher, id, maria, journalMarkInput(workTypes[0].ID, 4))
	require.NoError(t, err)
	require.NoError(t, f.env.Journal.SaveRecord(t.Context(), f.teacher, id, maria, journal.RecordInput{Comment: "молодец"}))

	_, err = f.env.DB.ExecContext(t.Context(), "DELETE FROM lessons WHERE id = ?", id)
	require.NoError(t, err)

	f.env.Journal.CleanupOrphans(t.Context())

	var marks, records int
	require.NoError(t, f.env.DB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM marks").Scan(&marks))
	require.NoError(t, f.env.DB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM lesson_students").Scan(&records))
	assert.Equal(t, 0, marks)
	assert.Equal(t, 0, records)
}
