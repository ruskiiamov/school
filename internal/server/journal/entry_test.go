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
	"github.com/ruskiiamov/school/internal/view"
)

func marksPath(lessonID, studentID int64) string {
	return lessonPath(lessonID, "/students/"+strconv.FormatInt(studentID, 10)+"/marks")
}

func typeField(id int64) string {
	return "type-" + strconv.FormatInt(id, 10)
}

func TestMarksEntry(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	cookies := []*http.Cookie{teacher}

	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")

	for _, id := range []int64{maria, petr} {
		require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, id))
	}

	workTypes := f.env.EnsureWorkTypes(t, f.teacher, f.subject)
	answer, test := workTypes[0], workTypes[1]

	lesson := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	response := servertest.Get(t, f.env.Handler, marksPath(lesson, maria), teacher, modalHeaders())
	assert.Equal(t, "false", response.Header().Get("HX-Push-Url"))
	window := response.Body.String()
	assert.Contains(t, window, "Иванова Мария · "+view.FormatShortDate(f.today))
	assert.Contains(t, window, `name="`+typeField(answer.ID)+`" value="" inputmode="numeric" maxlength="2" autocomplete="off" autofocus`)
	assert.Contains(t, window, `name="`+typeField(test.ID)+`"`)
	assert.Contains(t, window, ">"+answer.Name+"<")
	assert.Contains(t, window, `type="radio" name="absent" value="" checked`)
	assert.Contains(t, window, `type="radio" name="absent" value="1"`)
	assert.Contains(t, window, ">Был<")
	assert.Contains(t, window, ">Не был<")
	assert.Contains(t, window, `placeholder="Комментарий к уроку"`)
	assert.NotContains(t, window, `name="next"`)
	assert.NotContains(t, window, "<html")

	invalid := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria),
		url.Values{typeField(answer.ID): {"5"}, typeField(test.ID): {"7"}}, cookies, modalHeaders())
	require.Equal(t, http.StatusOK, invalid.Code)
	assert.Contains(t, invalid.Body.String(), "Оценка: цифра от 1 до 5, можно с + или −, или 0")
	assert.Contains(t, invalid.Body.String(), `name="`+typeField(test.ID)+`" value="7"`)

	marks, err := f.env.Journal.Marks(ctx, lesson)
	require.NoError(t, err)
	assert.Empty(t, marks)

	long := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria),
		url.Values{typeField(answer.ID): {"5"}, "comment": {strings.Repeat("а", 501)}}, cookies, modalHeaders())
	assert.Contains(t, long.Body.String(), "Комментарий не длиннее 500 символов")

	saved := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria),
		url.Values{typeField(answer.ID): {" 5 "}, typeField(test.ID): {"4"}, "absent": {"1"}, "comment": {"Опоздала"}}, cookies, modalHeaders())
	require.Equal(t, http.StatusOK, saved.Code)
	assert.Equal(t, "journal:close-modal", saved.Header().Get("HX-Trigger"))
	assert.Contains(t, saved.Body.String(), `id="journal-content" hx-swap-oob="true"`)
	assert.Contains(t, saved.Body.String(), "data-highlight")

	entry, err := f.env.Journal.MarksEntry(ctx, f.teacher, lesson, maria)
	require.NoError(t, err)
	assert.True(t, entry.Absent)
	assert.Equal(t, "Опоздала", entry.Comment)
	assert.Equal(t, map[int64]string{answer.ID: "5", test.ID: "4"}, entryValues(entry))

	updated := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria),
		url.Values{typeField(answer.ID): {""}, typeField(test.ID): {"3"}, "absent": {""}}, cookies, modalHeaders())
	require.Equal(t, http.StatusOK, updated.Code)
	assert.Equal(t, "journal:close-modal", updated.Header().Get("HX-Trigger"))

	entry, err = f.env.Journal.MarksEntry(ctx, f.teacher, lesson, maria)
	require.NoError(t, err)
	assert.False(t, entry.Absent)
	assert.Empty(t, entry.Comment)
	assert.Equal(t, map[int64]string{test.ID: "3"}, entryValues(entry))

	plain := servertest.PostForm(t, f.env.Handler, marksPath(lesson, petr), url.Values{typeField(answer.ID): {"2"}}, cookies, nil)
	servertest.AssertRedirect(t, plain, pairPath(f.class, f.subject)+"#lesson-"+strconv.FormatInt(lesson, 10))

	page := servertest.Get(t, f.env.Handler, marksPath(lesson, petr), teacher).Body.String()
	assert.Contains(t, page, "← К журналу")
	assert.Contains(t, page, `value="2"`)
	assert.NotContains(t, page, `hx-target="#modal-body"`)

	require.NoError(t, f.env.Journal.SetWorkTypeActive(ctx, f.teacher, test.ID, false))
	require.NoError(t, f.env.School.RemoveClassStudent(ctx, f.class, maria))

	outside := servertest.Get(t, f.env.Handler, marksPath(lesson, maria), teacher, modalHeaders()).Body.String()
	assert.Contains(t, outside, "Ученик не в классе")
	assert.Contains(t, outside, `name="`+typeField(test.ID)+`" value="3"`)
	assert.NotContains(t, outside, `name="`+typeField(answer.ID)+`"`)
	assert.NotContains(t, outside, `name="absent"`)
	assert.NotContains(t, outside, "Комментарий к уроку")

	cleared := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria), url.Values{typeField(test.ID): {""}, "absent": {"1"}}, cookies, modalHeaders())
	require.Equal(t, http.StatusOK, cleared.Code)

	_, err = f.env.Journal.MarksEntry(ctx, f.teacher, lesson, maria)
	require.ErrorIs(t, err, journal.ErrNotFound)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, marksPath(lesson, maria), teacher).Code)
}

func TestMarksEntryValuesAndComments(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	cookies := []*http.Cookie{teacher}

	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))

	workTypes := f.env.EnsureWorkTypes(t, f.teacher, f.subject)
	answer, test, written := workTypes[0], workTypes[1], workTypes[2]

	lesson := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	for _, raw := range []string{"6", "+", "-", "5++", "4*", "00", "45"} {
		invalid := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria), url.Values{typeField(answer.ID): {raw}}, cookies, modalHeaders())
		assert.Contains(t, invalid.Body.String(), "Оценка: цифра от 1 до 5, можно с + или −, или 0", raw)
	}

	orphan := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria),
		url.Values{typeField(answer.ID): {""}, "comment-" + strconv.FormatInt(answer.ID, 10): {"Без оценки"}}, cookies, modalHeaders())
	assert.Contains(t, orphan.Body.String(), "Комментарий без оценки")
	assert.Contains(t, orphan.Body.String(), `value="Без оценки"`)

	long := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria),
		url.Values{typeField(answer.ID): {"5"}, "comment-" + strconv.FormatInt(answer.ID, 10): {strings.Repeat("а", 201)}}, cookies, modalHeaders())
	assert.Contains(t, long.Body.String(), "Комментарий к оценке не длиннее 200 символов")

	saved := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria), url.Values{
		typeField(answer.ID):                        {"4+"},
		typeField(test.ID):                          {"3−"},
		typeField(written.ID):                       {"0"},
		"comment-" + strconv.FormatInt(test.ID, 10): {"  Ошибка   в задаче 3 "},
	}, cookies, modalHeaders())
	require.Equal(t, http.StatusOK, saved.Code)
	assert.Equal(t, "journal:close-modal", saved.Header().Get("HX-Trigger"))

	entry, err := f.env.Journal.MarksEntry(ctx, f.teacher, lesson, maria)
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{answer.ID: "4+", test.ID: "3-", written.ID: "0"}, entryValues(entry))
	assert.Equal(t, "Ошибка в задаче 3", entry.Types[1].Comment)

	window := servertest.Get(t, f.env.Handler, marksPath(lesson, maria), teacher, modalHeaders()).Body.String()
	assert.Contains(t, window, `name="`+typeField(test.ID)+`" value="3−"`)
	assert.Contains(t, window, `value="Ошибка в задаче 3"`)

	table := servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), teacher).Body.String()
	assert.Contains(t, table, "4+ 3− 0")
	assert.Contains(t, table, test.Name+": 3− (Ошибка в задаче 3)")
	assert.Contains(t, table, ">2,33<")

	cleared := servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria), url.Values{
		typeField(answer.ID):  {"4+"},
		typeField(test.ID):    {""},
		typeField(written.ID): {"0"},
	}, cookies, modalHeaders())
	require.Equal(t, http.StatusOK, cleared.Code)

	entry, err = f.env.Journal.MarksEntry(ctx, f.teacher, lesson, maria)
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{answer.ID: "4+", written.ID: "0"}, entryValues(entry))
	assert.Empty(t, entry.Types[1].Comment)
}

func TestMarksEntryForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))
	outsider := createUser(t, f.env, auth.RoleStudent, "outsider", "Чужой Ученик")
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	lesson := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, marksPath(lesson, outsider), teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, marksPath(lesson, outsider), url.Values{}, []*http.Cookie{teacher}, nil).Code)

	for _, cookie := range []*http.Cookie{f.env.LoginAs(t, "stranger"), f.env.LoginAs(t, "maria"), servertest.Login(t, f.env.Handler)} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, marksPath(lesson, maria), cookie).Code)
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, marksPath(lesson, maria), url.Values{}, []*http.Cookie{cookie}, nil).Code)
	}
}

func entryValues(entry journal.MarksEntry) map[int64]string {
	values := map[int64]string{}

	for _, workType := range entry.Types {
		if workType.Value != "" {
			values[workType.ID] = workType.Value
		}
	}

	return values
}
