package journal_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func markField(workTypeID, studentID int64) string {
	return journal.MarkKey{WorkTypeID: workTypeID, StudentID: studentID}.Field()
}

func cellHeaders() map[string]string {
	return map[string]string{"HX-Request": "true"}
}

func lessonMarks(t *testing.T, f fixture, lessonID int64) journal.LessonMarks {
	t.Helper()

	marks, err := f.env.Journal.LessonMarks(t.Context(), f.teacher, lessonID)
	require.NoError(t, err)

	return marks
}

type marksFixture struct {
	fixture
	cookie *http.Cookie
	lesson int64
	maria  int64
	petr   int64
	answer journal.WorkType
	test   journal.WorkType
}

func newMarksFixture(t *testing.T) marksFixture {
	t.Helper()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))

	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")

	for _, id := range []int64{maria, petr} {
		require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, id))
	}

	workTypes, err := f.env.Journal.WorkTypes(ctx, f.teacher, f.subject, false)
	require.NoError(t, err)

	cookie := f.env.LoginAs(t, "teacher")

	return marksFixture{
		fixture: f,
		cookie:  cookie,
		lesson:  openLesson(t, f.env, cookie, f.class, f.subject, f.today),
		maria:   maria,
		petr:    petr,
		answer:  workTypes[0],
		test:    workTypes[3],
	}
}

func TestLessonMarksExpand(t *testing.T) {
	t.Parallel()

	f := newMarksFixture(t)
	marksURL := lessonPath(f.lesson, "/marks")

	table := servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), f.cookie).Body.String()
	assert.Contains(t, table, `href="`+marksURL+`" hx-get="`+marksURL+`" hx-target="#lesson-`+strconv.FormatInt(f.lesson, 10)+`" hx-swap="outerHTML" data-marks-toggle aria-expanded="false"`)
	assert.NotContains(t, table, "data-marks-of")
	assert.Contains(t, table, "Чтобы поставить оценки всему классу")

	expanded := servertest.Get(t, f.env.Handler, marksURL, f.cookie, cellHeaders())
	require.Equal(t, http.StatusOK, expanded.Code)
	assert.Equal(t, "false", expanded.Header().Get("HX-Push-Url"))
	block := expanded.Body.String()
	assert.Contains(t, block, `id="lesson-`+strconv.FormatInt(f.lesson, 10)+`"`)
	assert.Contains(t, block, `aria-expanded="true"`)
	assert.Contains(t, block, `data-marks-of="lesson-`+strconv.FormatInt(f.lesson, 10)+`"`)
	assert.Contains(t, block, `name="`+markField(f.test.ID, f.petr)+`"`)
	assert.Contains(t, block, `aria-label="Козлов Пётр · Контрольная работа"`)
	assert.Contains(t, block, `hx-post="`+marksURL+`" hx-trigger="change" hx-target="closest td"`)
	assert.Contains(t, block, `placeholder="Новый тип работы"`)
	assert.NotContains(t, block, "<html")

	reopened := servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), f.cookie,
		map[string]string{"HX-Request": "true", "HX-Target": "journal-content", "X-Journal-Open": strconv.FormatInt(f.lesson, 10)})
	assert.Contains(t, reopened.Body.String(), `data-marks-of="lesson-`+strconv.FormatInt(f.lesson, 10)+`"`)

	page := servertest.Get(t, f.env.Handler, marksURL, f.cookie)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "← К журналу")
	assert.Contains(t, page.Body.String(), `href="`+studentMarksPath(f.lesson, f.maria)+`"`)
	assert.Contains(t, page.Body.String(), ">Сохранить<")
	assert.NotContains(t, page.Body.String(), `hx-trigger="change" hx-target="closest td"`)
}

func studentMarksPath(lessonID, studentID int64) string {
	return marksPath(lessonID, studentID)
}

func TestLessonMarksCellSave(t *testing.T) {
	t.Parallel()

	f := newMarksFixture(t)
	marksURL := lessonPath(f.lesson, "/marks")
	cookies := []*http.Cookie{f.cookie}

	f.env.SetCommentedMark(t, f.teacher, f.lesson, f.maria, f.test.ID, "5", "Отлично")

	saved := servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.test.ID, f.maria): {"4-"}}, cookies, cellHeaders())
	require.Equal(t, http.StatusOK, saved.Code)
	body := saved.Body.String()
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(f.lesson, 10)+`-`+markField(f.test.ID, f.maria)+`"`)
	assert.Contains(t, body, `value="4−"`)
	assert.Contains(t, body, `<tr id="lesson-`+strconv.FormatInt(f.lesson, 10)+`" hx-swap-oob="true"`)
	assert.Contains(t, body, `id="student-`+strconv.FormatInt(f.maria, 10)+`" hx-swap-oob="true"`)
	assert.Contains(t, body, ">3,50<")
	assert.NotContains(t, body, "<html")

	marks := lessonMarks(t, f.fixture, f.lesson)
	assert.Equal(t, journal.LessonMark{Value: "4-", Comment: "Отлично"}, marks.Marks[journal.MarkKey{WorkTypeID: f.test.ID, StudentID: f.maria}])

	invalid := servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.answer.ID, f.petr): {"7"}}, cookies, cellHeaders())
	require.Equal(t, http.StatusOK, invalid.Code)
	assert.Contains(t, invalid.Body.String(), `value="7"`)
	assert.Contains(t, invalid.Body.String(), `aria-invalid="true"`)
	assert.Contains(t, invalid.Body.String(), "Козлов Пётр · Ответ на уроке: Оценка: цифра от 1 до 5, можно с + или −, или 0")
	assert.NotContains(t, invalid.Body.String(), `<tr id="lesson-`)

	created := servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.answer.ID, f.petr): {"0"}}, cookies, cellHeaders())
	require.Equal(t, http.StatusOK, created.Code)

	cleared := servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.test.ID, f.maria): {""}}, cookies, cellHeaders())
	require.Equal(t, http.StatusOK, cleared.Code)

	marks = lessonMarks(t, f.fixture, f.lesson)
	assert.Equal(t, map[journal.MarkKey]journal.LessonMark{
		{WorkTypeID: f.answer.ID, StudentID: f.petr}: {Value: "0"},
	}, marks.Marks)

	plain := servertest.PostForm(t, f.env.Handler, marksURL, url.Values{
		markField(f.answer.ID, f.maria): {"5"},
		markField(f.test.ID, f.maria):   {"55"},
	}, cookies, nil)
	require.Equal(t, http.StatusOK, plain.Code)
	assert.Contains(t, plain.Body.String(), "Иванова Мария · Контрольная работа: Оценка")
	assert.Len(t, lessonMarks(t, f.fixture, f.lesson).Marks, 1)

	redirect := servertest.PostForm(t, f.env.Handler, marksURL, url.Values{
		markField(f.answer.ID, f.maria): {"5"},
		markField(f.test.ID, f.maria):   {"3+"},
	}, cookies, nil)
	servertest.AssertRedirect(t, redirect, pairPath(f.class, f.subject)+"#lesson-"+strconv.FormatInt(f.lesson, 10))
	assert.Len(t, lessonMarks(t, f.fixture, f.lesson).Marks, 3)
}

func TestLessonMarksRules(t *testing.T) {
	t.Parallel()

	f := newMarksFixture(t)
	ctx := t.Context()
	marksURL := lessonPath(f.lesson, "/marks")
	cookies := []*http.Cookie{f.cookie}

	f.env.SetMark(t, f.teacher, f.lesson, f.petr, f.test.ID, "4")
	require.NoError(t, f.env.School.RemoveClassStudent(ctx, f.class, f.petr))

	assert.Equal(t, http.StatusNotFound,
		servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.answer.ID, f.petr): {"5"}}, cookies, cellHeaders()).Code)
	assert.Equal(t, http.StatusOK,
		servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.test.ID, f.petr): {"3"}}, cookies, cellHeaders()).Code)

	other := createUser(t, f.env, auth.RoleTeacher, "other", "Кузнецов Виктор Павлович")
	foreign, err := f.env.Journal.WorkTypes(ctx, other, f.subject, false)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound,
		servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(foreign[0].ID, f.maria): {"5"}}, cookies, cellHeaders()).Code)

	require.NoError(t, f.env.Journal.SetWorkTypeActive(ctx, f.teacher, f.answer.ID, false))
	assert.Equal(t, http.StatusNotFound,
		servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.answer.ID, f.maria): {"5"}}, cookies, cellHeaders()).Code)

	marks := lessonMarks(t, f.fixture, f.lesson)
	assert.Equal(t, map[journal.MarkKey]journal.LessonMark{{WorkTypeID: f.test.ID, StudentID: f.petr}: {Value: "3"}}, marks.Marks)

	for _, cookie := range []*http.Cookie{f.env.LoginAs(t, "other"), f.env.LoginAs(t, "maria"), servertest.Login(t, f.env.Handler)} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, marksURL, cookie, cellHeaders()).Code)
		assert.Equal(t, http.StatusNotFound,
			servertest.PostForm(t, f.env.Handler, marksURL, url.Values{markField(f.test.ID, f.maria): {"5"}}, []*http.Cookie{cookie}, cellHeaders()).Code)
		assert.Equal(t, http.StatusNotFound,
			servertest.PostForm(t, f.env.Handler, lessonPath(f.lesson, "/work-types"), url.Values{"name": {"Проект"}}, []*http.Cookie{cookie}, nil).Code)
	}
}

func TestLessonMarksNewWorkType(t *testing.T) {
	t.Parallel()

	f := newMarksFixture(t)
	ctx := t.Context()
	cookies := []*http.Cookie{f.cookie}
	headers := map[string]string{"HX-Request": "true", "HX-Target": "journal-content", "X-Journal-Open": strconv.FormatInt(f.lesson, 10)}

	created := servertest.PostForm(t, f.env.Handler, lessonPath(f.lesson, "/work-types"), url.Values{"name": {"Диктант"}}, cookies, headers)
	require.Equal(t, http.StatusOK, created.Code)
	assert.Contains(t, created.Body.String(), `id="journal-content"`)
	assert.Contains(t, created.Body.String(), `title="Диктант"`)

	workTypes, err := f.env.Journal.WorkTypes(ctx, f.teacher, f.subject, false)
	require.NoError(t, err)
	assert.Equal(t, "Диктант", workTypes[len(workTypes)-1].Name)

	duplicate := servertest.PostForm(t, f.env.Handler, lessonPath(f.lesson, "/work-types"), url.Values{"name": {"Диктант"}}, cookies, headers)
	assert.Contains(t, duplicate.Body.String(), "Такой тип уже есть")
	assert.Contains(t, duplicate.Body.String(), `value="Диктант"`)

	servertest.AssertRedirect(t, servertest.PostForm(t, f.env.Handler, lessonPath(f.lesson, "/work-types"), url.Values{"name": {"Проект"}}, cookies, nil),
		lessonPath(f.lesson, "/marks"))

	substitute := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	_, err = f.env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substitute, StartDate: dateValue(f.today.AddDate(0, 0, 1)),
	})
	require.NoError(t, err)
	replaced := openLesson(t, f.env, f.env.LoginAs(t, "substitute"), f.class, f.subject, f.today.AddDate(0, 0, 1))

	block := servertest.Get(t, f.env.Handler, lessonPath(replaced, "/marks"), f.cookie, cellHeaders()).Body.String()
	assert.Contains(t, block, `data-marks-of="lesson-`+strconv.FormatInt(replaced, 10)+`"`)
	assert.NotContains(t, block, `placeholder="Новый тип работы"`)
	assert.NotContains(t, block, `title="Диктант"`)
	assert.Equal(t, http.StatusNotFound,
		servertest.PostForm(t, f.env.Handler, lessonPath(replaced, "/work-types"), url.Values{"name": {"Чужой"}}, cookies, headers).Code)
}
