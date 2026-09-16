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
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func markForm(workTypeID int64, value int, label string) url.Values {
	return url.Values{
		"work_type_id": {strconv.FormatInt(workTypeID, 10)},
		"value":        {strconv.Itoa(value)},
		"label":        {label},
	}
}

func studentQuery(studentID int64) string {
	return "?student=" + strconv.FormatInt(studentID, 10)
}

func TestLessonStudentsAndMarks(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")
	gone := createUser(t, f.env, auth.RoleStudent, "gone", "Ушедший Олег Олегович")
	outsider := createUser(t, f.env, auth.RoleStudent, "outsider", "Чужой Ученик")

	for _, id := range []int64{maria, petr, gone} {
		require.NoError(t, f.env.School.AddClassStudent(t.Context(), f.class, id))
	}

	require.NoError(t, f.env.Auth.SetUserActive(t.Context(), gone, false))

	workTypes, err := f.env.School.WorkTypes(t.Context(), false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(workTypes), 2)
	answer, test := workTypes[0], workTypes[1]

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	body := servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	assert.Contains(t, body, `id="lesson"`)
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, "Иванова М.")
	assert.Contains(t, body, "Козлов П.")
	assert.Contains(t, body, "удалён")
	assert.Contains(t, body, `href="`+lessonPath(id, "")+studentQuery(maria)+`" hx-get`)
	assert.Contains(t, body, "aria-current=\"true\"")
	assert.Contains(t, body, "Оценок пока нет")
	assert.Contains(t, body, `action="`+lessonPath(id, "/marks")+studentQuery(maria)+`"`)
	assert.Contains(t, body, `value="`+strconv.FormatInt(answer.ID, 10)+`">`+answer.Name+`<`)
	assert.NotContains(t, body, "Чужой Ученик")

	added := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks")+studentQuery(maria), markForm(answer.ID, 5, "  у  доски "), []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, added, lessonPath(id, "")+studentQuery(maria))

	added = servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks")+studentQuery(maria), markForm(test.ID, 4, ""), []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, added, lessonPath(id, "")+studentQuery(maria))

	marks, err := f.env.Journal.Marks(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, marks, 2)
	assert.Equal(t, "у доски", marks[0].Label)
	first := marks[0].ID

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria), teacher).Body.String()
	assert.Contains(t, body, ">5, 4<")
	assert.Contains(t, body, "у доски")
	assert.Contains(t, body, `hx-confirm="Удалить оценку 5 (`+answer.Name+`)?"`)
	assert.Contains(t, body, `href="`+lessonPath(id, "")+studentQuery(maria)+"&amp;mark="+strconv.FormatInt(first, 10)+`"`)
	assert.NotContains(t, body, `action="`+lessonPath(id, "/delete")+`"`)

	cases := []struct {
		name    string
		form    url.Values
		message string
	}{
		{"value out of range", markForm(answer.ID, 6, ""), "Оценка от 1 до 5"},
		{"no work type", markForm(0, 5, ""), "Выберите тип работы"},
		{"long label", markForm(answer.ID, 5, strings.Repeat("а", 101)), "Подпись не длиннее 100 символов"},
	}

	for _, tc := range cases {
		recorder := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks")+studentQuery(maria), tc.form, []*http.Cookie{teacher}, nil)
		assert.Equal(t, http.StatusOK, recorder.Code, tc.name)
		assert.Contains(t, recorder.Body.String(), tc.message, tc.name)
	}

	notMember := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks")+studentQuery(outsider), markForm(answer.ID, 5, ""), []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, notMember.Code)
	assert.Contains(t, notMember.Body.String(), "Ученик не в классе")

	edit := servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria)+"&mark="+strconv.FormatInt(first, 10), teacher).Body.String()
	assert.Contains(t, edit, `action="`+lessonPath(id, "/marks/"+strconv.FormatInt(first, 10))+studentQuery(maria)+`"`)
	assert.Contains(t, edit, `value="у доски"`)

	updated := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks/"+strconv.FormatInt(first, 10))+studentQuery(maria), markForm(test.ID, 3, "переписал"), []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, updated, lessonPath(id, "")+studentQuery(maria))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(maria), teacher).Body.String()
	assert.Contains(t, body, ">3, 4<")
	assert.Contains(t, body, "переписал")

	badUpdate := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks/"+strconv.FormatInt(first, 10))+studentQuery(maria), markForm(test.ID, 0, ""), []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, badUpdate.Code)
	assert.Contains(t, badUpdate.Body.String(), "Оценка от 1 до 5")

	fragment := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks")+studentQuery(petr), markForm(answer.ID, 2, ""), []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `id="lesson"`)
	assert.Contains(t, fragment.Body.String(), ">2<")
	assert.Contains(t, fragment.Body.String(), `id="lesson-actions" hx-swap-oob="true"`)
	assert.NotContains(t, fragment.Body.String(), "Удалить урок")
	assert.NotContains(t, fragment.Body.String(), "<html")

	notEmpty := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/delete"), nil, []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, notEmpty.Code)
	assert.Contains(t, notEmpty.Body.String(), "Урок с оценками или записями удалить нельзя")
	assert.Contains(t, notEmpty.Body.String(), "<html")

	notEmptyFragment := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/delete"), nil, []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, notEmptyFragment.Code)
	assert.Contains(t, notEmptyFragment.Body.String(), `id="lesson-actions"`)
	assert.Contains(t, notEmptyFragment.Body.String(), "Урок с оценками или записями удалить нельзя")
	assert.NotContains(t, notEmptyFragment.Body.String(), "Удалить урок")
	assert.NotContains(t, notEmptyFragment.Body.String(), "<html")

	require.NoError(t, f.env.School.RemoveClassStudent(t.Context(), f.class, petr))
	require.NoError(t, f.env.School.SetWorkTypeActive(t.Context(), answer.ID, false))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, "")+studentQuery(petr), teacher).Body.String()
	assert.Contains(t, body, "не в классе")
	assert.Contains(t, body, answer.Name)
	assert.NotContains(t, body, `action="`+lessonPath(id, "/marks")+studentQuery(petr)+`"`)
	assert.NotContains(t, body, `value="`+strconv.FormatInt(answer.ID, 10)+`">`+answer.Name+`<`)

	removed := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks/"+strconv.FormatInt(first, 10)+"/delete")+studentQuery(maria), nil, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, removed, lessonPath(id, "")+studentQuery(maria))

	marks, err = f.env.Journal.Marks(t.Context(), id)
	require.NoError(t, err)
	assert.Len(t, marks, 2)

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks/"+strconv.FormatInt(first, 10)+"/delete"), nil, []*http.Cookie{teacher}, nil).Code)

	other := openLesson(t, f.env, teacher, f.class, f.subject, f.today.AddDate(0, 0, 1))
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(other, "/marks/"+strconv.FormatInt(marks[0].ID, 10)+"/delete"), nil, []*http.Cookie{teacher}, nil).Code)
}

func TestLessonMarksForeignTeacher(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(t.Context(), f.class, maria))
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	stranger := f.env.LoginAs(t, "stranger")

	workTypes, err := f.env.School.WorkTypes(t.Context(), false)
	require.NoError(t, err)

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	markID, err := f.env.Journal.AddMark(t.Context(), f.teacher, id, maria, journalMarkInput(workTypes[0].ID, 5))
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks")+studentQuery(maria), markForm(workTypes[0].ID, 5, ""), []*http.Cookie{stranger}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks/"+strconv.FormatInt(markID, 10)), markForm(workTypes[0].ID, 4, ""), []*http.Cookie{stranger}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(id, "/marks/"+strconv.FormatInt(markID, 10)+"/delete"), nil, []*http.Cookie{stranger}, nil).Code)
}
