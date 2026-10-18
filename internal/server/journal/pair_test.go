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
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/view"
)

func pairPath(classID, subjectID int64) string {
	return "/journal/class/" + strconv.FormatInt(classID, 10) + "/subject/" + strconv.FormatInt(subjectID, 10)
}

func TestPairJournalTable(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")

	body := servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), teacher).Body.String()
	assert.Contains(t, body, "7А · Алгебра · "+school.YearName(f.env.School.CurrentYear()))
	assert.Contains(t, body, `href="/" class`)
	assert.Contains(t, body, "Уроков пока нет")
	assert.Contains(t, body, "Всего: 0 уроков")

	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	petr := createUser(t, f.env, auth.RoleStudent, "petr", "Козлов Пётр Ильич")
	gone := createUser(t, f.env, auth.RoleStudent, "gone", "Ушедший Олег Олегович")

	for _, id := range []int64{maria, petr, gone} {
		require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, id))
	}

	workTypes, err := f.env.School.WorkTypes(ctx, false)
	require.NoError(t, err)
	answer, test := workTypes[0], workTypes[1]

	today := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	earlier := openLesson(t, f.env, teacher, f.class, f.subject, f.today.AddDate(0, 0, -7))
	require.NoError(t, f.env.Journal.UpdateTopic(ctx, f.teacher, earlier, "Квадратные уравнения"))
	require.NoError(t, f.env.Journal.SaveHomework(ctx, f.teacher, earlier, journalHomework("Упр. 12")))

	f.env.SetMark(t, f.teacher, today, maria, test.ID, 4)
	f.env.SetMark(t, f.teacher, today, maria, answer.ID, 5)
	f.env.SetMark(t, f.teacher, earlier, maria, answer.ID, 4)
	f.env.SetMark(t, f.teacher, earlier, gone, answer.ID, 3)
	f.env.SetRecord(t, f.teacher, today, petr, true, "Болел")
	require.NoError(t, f.env.School.RemoveClassStudent(ctx, f.class, gone))
	require.NoError(t, f.env.Auth.SetUserActive(ctx, petr, false))

	body = servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), teacher).Body.String()
	assert.NotContains(t, body, "Уроков пока нет")
	assert.Contains(t, body, "Всего: 2 урока")
	assert.Contains(t, body, "Как работать с журналом")
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(today, 10)+`"`)
	assert.Less(t, strings.Index(body, `id="lesson-`+strconv.FormatInt(today, 10)+`"`), strings.Index(body, `id="lesson-`+strconv.FormatInt(earlier, 10)+`"`))
	assert.Contains(t, body, `name="date" value="`+inputDate(f.today)+`"`)
	assert.Contains(t, body, "Квадратные уравнения")

	assert.Less(t, strings.Index(body, "<span>Иванова</span> <span>Мария</span>"), strings.Index(body, "<span>Козлов</span> <span>Пётр</span>"))
	assert.Less(t, strings.Index(body, "<span>Козлов</span>"), strings.Index(body, "<span>Ушедший</span>"))
	assert.Contains(t, body, `title="Козлов Пётр Ильич (удалён)"`)
	assert.Contains(t, body, `title="Ушедший Олег Олегович (не в классе)"`)
	assert.Contains(t, body, ">4,33<")
	assert.Contains(t, body, ">1×Н<")
	assert.Contains(t, body, ">0×Н<")

	mariaToday := lessonPath(today, "/students/"+strconv.FormatInt(maria, 10)+"/marks")
	assert.Contains(t, body, `href="`+mariaToday+`" title="`+answer.Name+": 5 · "+test.Name+`: 4"`)
	assert.Contains(t, body, `title="Отсутствовал · Болел"`)
	assert.Contains(t, body, "<span>•</span>")
	assert.Equal(t, 1, strings.Count(body, "bg-emerald-50"))
	assert.NotContains(t, body, "замена ·")
}

func TestPairJournalSubstitution(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))

	substitute := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	start := f.today.AddDate(0, 0, -3)
	_, err := f.env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substitute, StartDate: dateValue(start), EndDate: dateValue(f.today.AddDate(0, 0, 1)),
	})
	require.NoError(t, err)
	substituteCookie := f.env.LoginAs(t, "substitute")

	own := openLesson(t, f.env, teacher, f.class, f.subject, f.today)
	replaced := openLesson(t, f.env, substituteCookie, f.class, f.subject, f.today.AddDate(0, 0, -2))

	body := servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), teacher).Body.String()
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(own, 10)+`"`)
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(replaced, 10)+`"`)
	assert.Contains(t, body, `title="Урок провёл Кузнецов В. П."`)
	assert.NotContains(t, body, "Ваши замены")

	body = servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), substituteCookie).Body.String()
	assert.Contains(t, body, "Ваши замены: "+view.FormatPeriod(start, f.today.AddDate(0, 0, 1)))
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(replaced, 10)+`"`)
	assert.NotContains(t, body, `id="lesson-`+strconv.FormatInt(own, 10)+`"`)
	assert.NotContains(t, body, "замена ·")
}

func TestPairJournalForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	createUser(t, f.env, auth.RoleStudent, "student", "Иванова Мария Петровна")

	path := pairPath(f.class, f.subject)
	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, path, f.env.LoginAs(t, "teacher")).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, path, f.env.LoginAs(t, "stranger")).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, path, f.env.LoginAs(t, "student")).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, path, servertest.Login(t, f.env.Handler)).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, pairPath(f.class+100, f.subject), f.env.LoginAs(t, "teacher")).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/journal/class/x/subject/1", f.env.LoginAs(t, "teacher")).Code)
	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, path), "/login")
}

func TestPairJournalLessons(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	maria := createUser(t, f.env, auth.RoleStudent, "maria", "Иванова Мария Петровна")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, maria))

	path := pairPath(f.class, f.subject)
	create := path + "/lessons"
	htmx := map[string]string{"HX-Request": "true", "HX-Target": "journal-content"}
	start, end := f.env.School.YearBounds(f.env.School.CurrentYear())

	body := servertest.Get(t, f.env.Handler, path, teacher).Body.String()
	assert.Contains(t, body, `action="`+create+`"`)
	assert.Contains(t, body, `name="date" value="" aria-label="Дата нового урока"`)
	assert.NotContains(t, body, ">Сегодня<")

	created := servertest.PostForm(t, f.env.Handler, create, url.Values{"date": {dateValue(f.today)}}, []*http.Cookie{teacher}, nil)
	require.Equal(t, http.StatusSeeOther, created.Code)
	lessons := pairLessons(t, f)
	require.Len(t, lessons, 1)
	today := lessons[0].ID
	servertest.AssertRedirect(t, created, path+"#lesson-"+strconv.FormatInt(today, 10))

	fragment := servertest.PostForm(t, f.env.Handler, create, url.Values{"date": {dateValue(f.today.AddDate(0, 0, -1))}}, []*http.Cookie{teacher}, htmx)
	require.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `id="journal-content"`)
	assert.Contains(t, fragment.Body.String(), "data-highlight")
	assert.Contains(t, fragment.Body.String(), "Всего: 2 урока")
	assert.NotContains(t, fragment.Body.String(), "<html")

	cases := []struct {
		name    string
		date    string
		message string
	}{
		{"empty", "", "Укажите дату"},
		{"garbage", "завтра", "Неверная дата"},
		{"taken", inputDate(f.today), "Урок на " + view.FormatShortDate(f.today) + " уже есть"},
		{"before year", dateValue(start.AddDate(0, 0, -1)), "Дата вне учебного года " + school.YearName(f.env.School.CurrentYear())},
		{"after year", dateValue(end.AddDate(0, 0, 1)), "Дата вне учебного года"},
	}

	for _, tc := range cases {
		recorder := servertest.PostForm(t, f.env.Handler, create, url.Values{"date": {tc.date}}, []*http.Cookie{teacher}, htmx)
		require.Equal(t, http.StatusOK, recorder.Code, tc.name)
		assert.Contains(t, recorder.Body.String(), tc.message, tc.name)
		assert.Contains(t, recorder.Body.String(), `name="date" value="`+html.EscapeString(tc.date)+`" aria-label="Дата нового урока"`, tc.name)
	}

	f.env.SetMark(t, f.teacher, today, maria, firstWorkType(t, f), 5)

	moved := f.today.AddDate(0, 0, -3)
	recorder := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/date"), url.Values{"date": {dateValue(moved)}}, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, recorder, path+"#lesson-"+strconv.FormatInt(today, 10))

	lesson, err := f.env.Journal.LessonForTeacher(ctx, f.teacher, today)
	require.NoError(t, err)
	assert.True(t, lesson.Date.Equal(moved))

	marks, err := f.env.Journal.Marks(ctx, today)
	require.NoError(t, err)
	assert.Len(t, marks, 1)

	conflict := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/date"), url.Values{"date": {dateValue(f.today.AddDate(0, 0, -1))}}, []*http.Cookie{teacher}, htmx)
	require.Equal(t, http.StatusOK, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "Урок на "+view.FormatShortDate(f.today.AddDate(0, 0, -1))+" уже есть")
	assert.Contains(t, conflict.Body.String(), `value="`+inputDate(f.today.AddDate(0, 0, -1))+`" aria-label="Дата урока"`)

	same := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/date"), url.Values{"date": {dateValue(moved)}}, []*http.Cookie{teacher}, htmx)
	require.Equal(t, http.StatusOK, same.Code)
	assert.NotContains(t, same.Body.String(), "уже есть")
	assert.Contains(t, same.Body.String(), `data-confirm-empty="Удалить урок `+view.FormatShortDate(moved)+`?"`)

	refused := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/date"), url.Values{"date": {""}}, []*http.Cookie{teacher}, htmx)
	require.Equal(t, http.StatusOK, refused.Code)
	assert.Contains(t, refused.Body.String(), "Урок с оценками или записями удалить нельзя")
	assert.Contains(t, refused.Body.String(), `value="`+inputDate(moved)+`" aria-label="Дата урока"`)

	lessons = pairLessons(t, f)
	require.Len(t, lessons, 2)

	var empty int64
	for _, item := range lessons {
		if item.ID != today {
			empty = item.ID
		}
	}

	cleared := servertest.PostForm(t, f.env.Handler, lessonPath(empty, "/date"), url.Values{"date": {""}}, []*http.Cookie{teacher}, htmx)
	require.Equal(t, http.StatusOK, cleared.Code)
	assert.Contains(t, cleared.Body.String(), "Всего: 1 урок")
	assert.NotContains(t, cleared.Body.String(), `id="lesson-`+strconv.FormatInt(empty, 10)+`"`)

	_, err = f.env.Journal.LessonForTeacher(ctx, f.teacher, empty)
	require.ErrorIs(t, err, journal.ErrNotFound)

	topicTarget := map[string]string{"HX-Request": "true", "HX-Target": "topic-" + strconv.FormatInt(today, 10) + "-error"}
	topic := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/topic"), url.Values{"topic": {"Дроби"}, "from": {"journal"}}, []*http.Cookie{teacher}, topicTarget)
	require.Equal(t, http.StatusOK, topic.Code)
	assert.Contains(t, topic.Body.String(), `id="topic-`+strconv.FormatInt(today, 10)+`-error" data-error`)
	assert.NotContains(t, topic.Body.String(), "journal-content")

	long := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/topic"), url.Values{"topic": {strings.Repeat("а", 201)}, "from": {"journal"}}, []*http.Cookie{teacher}, topicTarget)
	assert.Contains(t, long.Body.String(), "Тема не длиннее 200 символов")

	lesson, err = f.env.Journal.LessonForTeacher(ctx, f.teacher, today)
	require.NoError(t, err)
	assert.Equal(t, "Дроби", lesson.Topic)

	plain := servertest.PostForm(t, f.env.Handler, lessonPath(today, "/topic"), url.Values{"topic": {"Дроби и проценты"}, "from": {"journal"}}, []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, plain, path+"#lesson-"+strconv.FormatInt(today, 10))
}

func TestPairJournalLessonsSubstitute(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")
	substitute := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	_, err := f.env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substitute,
		StartDate: dateValue(f.today.AddDate(0, 0, -2)), EndDate: dateValue(f.today),
	})
	require.NoError(t, err)
	substituteCookie := f.env.LoginAs(t, "substitute")
	htmx := map[string]string{"HX-Request": "true", "HX-Target": "journal-content"}
	create := pairPath(f.class, f.subject) + "/lessons"

	main := openLesson(t, f.env, teacher, f.class, f.subject, f.today)

	cases := []struct {
		date    string
		message string
	}{
		{dateValue(f.today.AddDate(0, 0, -5)), "На эту дату замена не действует"},
		{dateValue(f.today), "Урок на эту дату уже создал другой учитель"},
	}

	for _, tc := range cases {
		recorder := servertest.PostForm(t, f.env.Handler, create, url.Values{"date": {tc.date}}, []*http.Cookie{substituteCookie}, htmx)
		require.Equal(t, http.StatusOK, recorder.Code, tc.date)
		assert.Contains(t, recorder.Body.String(), tc.message, tc.date)
	}

	own := servertest.PostForm(t, f.env.Handler, create, url.Values{"date": {dateValue(f.today.AddDate(0, 0, -1))}}, []*http.Cookie{substituteCookie}, nil)
	require.Equal(t, http.StatusSeeOther, own.Code)

	lessons := pairLessons(t, f)
	require.Len(t, lessons, 2)
	replaced := lessons[1].ID

	outside := servertest.PostForm(t, f.env.Handler, lessonPath(replaced, "/date"), url.Values{"date": {dateValue(f.today.AddDate(0, 0, 3))}}, []*http.Cookie{substituteCookie}, htmx)
	assert.Contains(t, outside.Body.String(), "На эту дату замена не действует")

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(main, "/date"),
		url.Values{"date": {dateValue(f.today.AddDate(0, 0, -2))}}, []*http.Cookie{substituteCookie}, nil).Code)

	moved := servertest.PostForm(t, f.env.Handler, lessonPath(replaced, "/date"), url.Values{"date": {dateValue(f.today.AddDate(0, 0, 2))}}, []*http.Cookie{teacher}, nil)
	require.Equal(t, http.StatusSeeOther, moved.Code)
}

func TestPairJournalLessonsForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	createUser(t, f.env, auth.RoleTeacher, "stranger", "Посторонний Учитель")
	createUser(t, f.env, auth.RoleStudent, "student", "Иванова Мария Петровна")
	lesson := openLesson(t, f.env, f.env.LoginAs(t, "teacher"), f.class, f.subject, f.today)

	form := url.Values{"date": {dateValue(f.today.AddDate(0, 0, -1))}}
	create := pairPath(f.class, f.subject) + "/lessons"

	for _, cookie := range []*http.Cookie{f.env.LoginAs(t, "stranger"), f.env.LoginAs(t, "student"), servertest.Login(t, f.env.Handler)} {
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, create, form, []*http.Cookie{cookie}, nil).Code)
		assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, lessonPath(lesson, "/date"), form, []*http.Cookie{cookie}, nil).Code)
	}

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, pairPath(f.class+100, f.subject)+"/lessons", form,
		[]*http.Cookie{f.env.LoginAs(t, "teacher")}, nil).Code)
}

func firstWorkType(t *testing.T, f fixture) int64 {
	t.Helper()

	workTypes, err := f.env.School.WorkTypes(t.Context(), false)
	require.NoError(t, err)

	return workTypes[0].ID
}
