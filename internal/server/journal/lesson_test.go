package journal_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestJournalOpenLessonTopicDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")

	body := servertest.Get(t, f.env.Handler, "/journal", teacher).Body.String()
	assert.Contains(t, body, "Вам пока не назначены классы и предметы")
	assert.Contains(t, body, "Пока нет уроков")
	assert.NotContains(t, body, `name="pair"`)

	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	body = servertest.Get(t, f.env.Handler, "/journal", teacher).Body.String()
	assert.Contains(t, body, `<option value="`+pairValue(f.class, f.subject)+`">7А · Алгебра</option>`)
	assert.Contains(t, body, `name="date" value="`+dateValue(f.today)+`"`)

	id := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	assert.Equal(t, id, openLesson(t, f.env, teacher, f.class, f.subject, f.today))

	body = servertest.Get(t, f.env.Handler, "/journal", teacher).Body.String()
	assert.Contains(t, body, `href="`+lessonPath(id, "")+`"`)
	assert.Contains(t, body, f.today.Format("02.01.2006"))
	assert.NotContains(t, body, "Пока нет уроков")

	lesson := servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher)
	require.Equal(t, http.StatusOK, lesson.Code)
	assert.Contains(t, lesson.Body.String(), "7А · Алгебра · "+f.today.Format("02.01.2006"))
	assert.Contains(t, lesson.Body.String(), `action="`+lessonPath(id, "/topic")+`"`)
	assert.Contains(t, lesson.Body.String(), `action="`+lessonPath(id, "/delete")+`"`)
	assert.Contains(t, lesson.Body.String(), "Удалить урок")

	topic := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/topic"), url.Values{"topic": {"  Квадратные   уравнения "}}, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, topic, lessonPath(id, ""))

	body = servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Body.String()
	assert.Contains(t, body, `value="Квадратные уравнения"`)

	body = servertest.Get(t, f.env.Handler, "/journal", teacher).Body.String()
	assert.Contains(t, body, "Квадратные уравнения")

	emptyFragment := servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, emptyFragment.Code)
	assert.Contains(t, emptyFragment.Body.String(), `id="lesson-actions" hx-swap-oob="true"`)
	assert.Contains(t, emptyFragment.Body.String(), "Удалить урок")

	fragment := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/topic"), url.Values{"topic": {"Повторение"}}, []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `id="lesson-topic"`)
	assert.Contains(t, fragment.Body.String(), `value="Повторение"`)
	assert.NotContains(t, fragment.Body.String(), "<html")

	long := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/topic"), url.Values{"topic": {strings.Repeat("а", 201)}}, []*http.Cookie{teacher}, nil)
	assert.Equal(t, http.StatusOK, long.Code)
	assert.Contains(t, long.Body.String(), "Тема не длиннее 200 символов")

	removed := servertest.PostForm(t, f.env.Handler, lessonPath(id, "/delete"), nil, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, removed, "/journal")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, lessonPath(id, ""), teacher).Code)

	body = servertest.Get(t, f.env.Handler, "/journal", teacher).Body.String()
	assert.Contains(t, body, "Пока нет уроков")
}

func TestJournalOpenLessonErrors(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	teacher := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	cases := []struct {
		name    string
		form    url.Values
		message string
	}{
		{"unknown pair", url.Values{"pair": {"99-99"}, "date": {dateValue(f.today)}}, "Выберите класс и предмет из списка"},
		{"empty date", url.Values{"pair": {pairValue(f.class, f.subject)}, "date": {""}}, "Укажите дату"},
		{"bad date", url.Values{"pair": {pairValue(f.class, f.subject)}, "date": {"15.09.2026"}}, "Неверная дата"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recorder := servertest.PostForm(t, f.env.Handler, "/journal", tc.form, []*http.Cookie{teacher}, nil)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), tc.message)
		})
	}
}

func TestJournalSubstituteRights(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	main := f.env.LoginAs(t, "teacher")
	require.NoError(t, f.env.School.AssignTeacher(t.Context(), f.class, f.subject, f.teacher))

	substituteID := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	substitute := f.env.LoginAs(t, "substitute")
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	stranger := f.env.LoginAs(t, "stranger")

	body := servertest.Get(t, f.env.Handler, "/journal", substitute).Body.String()
	assert.Contains(t, body, "Вам пока не назначены классы и предметы")

	_, err := f.env.School.CreateSubstitution(t.Context(), school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substituteID,
		StartDate: dateValue(f.today.AddDate(0, 0, -1)), EndDate: dateValue(f.today.AddDate(0, 0, 1)),
	})
	require.NoError(t, err)

	body = servertest.Get(t, f.env.Handler, "/journal", substitute).Body.String()
	assert.Contains(t, body, "7А · Алгебра")

	outside := servertest.PostForm(t, f.env.Handler, "/journal",
		url.Values{"pair": {pairValue(f.class, f.subject)}, "date": {dateValue(f.today.AddDate(0, 0, 5))}},
		[]*http.Cookie{substitute}, nil)
	assert.Equal(t, http.StatusOK, outside.Code)
	assert.Contains(t, outside.Body.String(), "На эту дату замена не действует")

	lessonBySubstitute := openLesson(t, f.env, substitute, f.class, f.subject, f.today)
	assert.Equal(t, lessonBySubstitute, openLesson(t, f.env, main, f.class, f.subject, f.today))
	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, lessonPath(lessonBySubstitute, ""), main).Code)

	lessonByMain := openLesson(t, f.env, main, f.class, f.subject, f.today.AddDate(0, 0, 1))

	taken := servertest.PostForm(t, f.env.Handler, "/journal",
		url.Values{"pair": {pairValue(f.class, f.subject)}, "date": {dateValue(f.today.AddDate(0, 0, 1))}},
		[]*http.Cookie{substitute}, nil)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.Contains(t, taken.Body.String(), "Урок на эту дату уже создал другой учитель")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, lessonPath(lessonByMain, ""), substitute).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(lessonByMain, "/topic"), url.Values{"topic": {"x"}}, []*http.Cookie{substitute}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(lessonByMain, "/delete"), nil, []*http.Cookie{substitute}, nil).Code)

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, lessonPath(lessonBySubstitute, ""), stranger).Code)

	body = servertest.Get(t, f.env.Handler, "/journal", substitute).Body.String()
	assert.Contains(t, body, `href="`+lessonPath(lessonBySubstitute, "")+`"`)
	assert.NotContains(t, body, `href="`+lessonPath(lessonByMain, "")+`"`)

	body = servertest.Get(t, f.env.Handler, "/journal", main).Body.String()
	assert.Contains(t, body, `href="`+lessonPath(lessonBySubstitute, "")+`"`)
	assert.Contains(t, body, `href="`+lessonPath(lessonByMain, "")+`"`)
}

func TestJournalHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	createUser(t, f.env, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	student := f.env.LoginAs(t, "student")
	admin := servertest.Login(t, f.env.Handler)

	for _, cookie := range []*http.Cookie{student, admin} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/journal", cookie).Code)
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, "/journal", url.Values{}, []*http.Cookie{cookie}, nil).Code)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, lessonPath(1, ""), cookie).Code)
	}

	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, "/journal"), "/login")
}
