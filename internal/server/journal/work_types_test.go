package journal_test

import (
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

func workTypesURL(subjectID int64, suffix string) string {
	return "/journal/subjects/" + strconv.FormatInt(subjectID, 10) + "/work-types" + suffix
}

func workTypeURL(subjectID, id int64, suffix string) string {
	return workTypesURL(subjectID, "/"+strconv.FormatInt(id, 10)+suffix)
}

func TestTeacherWorkTypes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	geometry, err := f.env.School.CreateSubject(ctx, school.SubjectInput{Name: "Геометрия"})
	require.NoError(t, err)
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, geometry, f.teacher))

	teacher := f.env.LoginAs(t, "teacher")
	cookies := []*http.Cookie{teacher}

	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, "/journal/work-types", teacher), workTypesURL(f.subject, ""))

	body := servertest.Get(t, f.env.Handler, workTypesURL(f.subject, ""), teacher).Body.String()
	assert.Contains(t, body, `href="/journal/work-types" aria-current="page"`)
	assert.Contains(t, body, `href="`+workTypesURL(f.subject, "")+`" aria-current="page"`)
	assert.Contains(t, body, `href="`+workTypesURL(geometry, "")+`"`)
	assert.Contains(t, body, "общий для всех ваших классов")
	assert.Contains(t, body, ">Ответ на уроке<")
	assert.Contains(t, body, ">Домашняя работа (устная)<")

	created := servertest.PostForm(t, f.env.Handler, workTypesURL(f.subject, ""), url.Values{"name": {" Диктант "}}, cookies, nil)
	servertest.AssertRedirect(t, created, workTypesURL(f.subject, ""))

	duplicate := servertest.PostForm(t, f.env.Handler, workTypesURL(f.subject, ""), url.Values{"name": {"Диктант"}}, cookies, nil)
	require.Equal(t, http.StatusOK, duplicate.Code)
	assert.Contains(t, duplicate.Body.String(), "Такой тип уже есть")

	empty := servertest.PostForm(t, f.env.Handler, workTypesURL(f.subject, ""), url.Values{"name": {""}}, cookies, nil)
	assert.Contains(t, empty.Body.String(), "Укажите название")

	algebra, err := f.env.Journal.WorkTypes(ctx, f.teacher, f.subject, false)
	require.NoError(t, err)
	require.Len(t, algebra, 7)
	dictation := algebra[6]
	assert.Equal(t, "Диктант", dictation.Name)

	geometryTypes, err := f.env.Journal.WorkTypes(ctx, f.teacher, geometry, false)
	require.NoError(t, err)
	assert.Len(t, geometryTypes, 6)

	servertest.AssertRedirect(t, servertest.PostForm(t, f.env.Handler, workTypeURL(f.subject, dictation.ID, "/up"), nil, cookies, nil), workTypesURL(f.subject, ""))
	servertest.AssertRedirect(t, servertest.PostForm(t, f.env.Handler, workTypeURL(f.subject, dictation.ID, ""), url.Values{"name": {"Словарный диктант"}}, cookies, nil), workTypesURL(f.subject, ""))
	servertest.AssertRedirect(t, servertest.PostForm(t, f.env.Handler, workTypeURL(f.subject, algebra[0].ID, "/deactivate"), nil, cookies, nil), workTypesURL(f.subject, ""))

	algebra, err = f.env.Journal.WorkTypes(ctx, f.teacher, f.subject, false)
	require.NoError(t, err)
	require.Len(t, algebra, 6)
	assert.Equal(t, "Словарный диктант", algebra[4].Name)

	fragment := servertest.PostForm(t, f.env.Handler, workTypesURL(f.subject, "?inactive=1"), url.Values{"name": {"Проект"}}, cookies, map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `id="work-types"`)
	assert.Contains(t, fragment.Body.String(), ">Ответ на уроке<")
	assert.NotContains(t, fragment.Body.String(), "<html")

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, workTypeURL(geometry, dictation.ID, "/up"), nil, cookies, nil).Code)
}

func TestTeacherWorkTypesForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))

	other := createUser(t, f.env, auth.RoleTeacher, "other", "Кузнецов Виктор Павлович")
	history, err := f.env.School.CreateSubject(ctx, school.SubjectInput{Name: "История"})
	require.NoError(t, err)
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, history, other))

	types, err := f.env.Journal.WorkTypes(ctx, f.teacher, f.subject, true)
	require.NoError(t, err)

	otherCookie := f.env.LoginAs(t, "other")
	otherCookies := []*http.Cookie{otherCookie}

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, workTypesURL(f.subject, ""), otherCookie).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, workTypesURL(f.subject, ""), url.Values{"name": {"Проект"}}, otherCookies, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, workTypeURL(f.subject, types[0].ID, ""), url.Values{"name": {"Чужое"}}, otherCookies, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, workTypeURL(f.subject, types[0].ID, "/deactivate"), nil, otherCookies, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, f.env.Handler, workTypeURL(history, types[0].ID, "/down"), nil, otherCookies, nil).Code)

	unchanged, err := f.env.Journal.WorkTypes(ctx, f.teacher, f.subject, false)
	require.NoError(t, err)
	assert.Equal(t, types[0].Name, unchanged[0].Name)

	lonely := createUser(t, f.env, auth.RoleTeacher, "lonely", "Петрова Ольга Ивановна")
	require.NotZero(t, lonely)
	page := servertest.Get(t, f.env.Handler, "/journal/work-types", f.env.LoginAs(t, "lonely"))
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "Вам пока не назначены предметы")

	student := createUser(t, f.env, auth.RoleStudent, "student", "Иванова Мария Петровна")
	require.NotZero(t, student)

	for _, cookie := range []*http.Cookie{f.env.LoginAs(t, "student"), servertest.Login(t, f.env.Handler)} {
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/journal/work-types", cookie).Code)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, workTypesURL(f.subject, ""), cookie).Code)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/admin/work-types", cookie).Code)
	}

	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, "/journal/work-types"), "/login")
}

func TestMarksEntryUsesLessonAuthorTypes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))

	substitute := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	_, err := f.env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substitute, StartDate: dateValue(f.today),
	})
	require.NoError(t, err)

	student := createUser(t, f.env, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, student))

	_, err = f.env.Journal.CreateWorkType(ctx, f.teacher, f.subject, "Диктант")
	require.NoError(t, err)
	_, err = f.env.Journal.CreateWorkType(ctx, substitute, f.subject, "Проект")
	require.NoError(t, err)

	teacher := f.env.LoginAs(t, "teacher")
	own := openLesson(t, f.env, teacher, f.class, f.subject, f.today.AddDate(0, 0, -1))
	replaced := openLesson(t, f.env, f.env.LoginAs(t, "substitute"), f.class, f.subject, f.today)

	window := servertest.Get(t, f.env.Handler, marksPath(own, student), teacher, modalHeaders()).Body.String()
	assert.Contains(t, window, ">Диктант<")
	assert.NotContains(t, window, ">Проект<")

	window = servertest.Get(t, f.env.Handler, marksPath(replaced, student), teacher, modalHeaders()).Body.String()
	assert.Contains(t, window, ">Проект<")
	assert.NotContains(t, window, ">Диктант<")
}
