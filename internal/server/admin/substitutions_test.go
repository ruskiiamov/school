package admin_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

const substitutionsPath = "/admin/substitutions"

func substitutionForm(classID, subjectID, teacherID int64, start, end time.Time) url.Values {
	form := url.Values{
		"class_id":   {strconv.FormatInt(classID, 10)},
		"subject_id": {strconv.FormatInt(subjectID, 10)},
		"teacher_id": {strconv.FormatInt(teacherID, 10)},
		"start_date": {start.Format(validation.DateLayout)},
		"end_date":   {""},
	}

	if !end.IsZero() {
		form.Set("end_date", end.Format(validation.DateLayout))
	}

	return form
}

func substitutionPath(id int64, suffix string) string {
	return substitutionsPath + "/" + strconv.FormatInt(id, 10) + suffix
}

func TestSubstitutionsCreateEditDelete(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	current := env.School.CurrentYear()
	today := env.School.Today()

	class := createClass(t, env, admin, current, "7А")
	past := createPastClass(t, env, current-1, "6А")
	algebra := createSubject(t, env, admin, "Алгебра")
	viktor, _ := createUserVia(t, env, admin, "/admin/teachers", servertest.NameForm("Кузнецов Виктор"))
	gone, _ := createUserVia(t, env, admin, "/admin/teachers", servertest.NameForm("Ушедший Учитель"))
	require.NoError(t, env.Auth.SetUserActive(t.Context(), gone, false))

	body := servertest.Get(t, env.Handler, substitutionsPath, admin).Body.String()
	assert.Contains(t, body, "Пока нет замен")
	assert.Contains(t, body, `value="`+strconv.FormatInt(class, 10)+`">7А<`)
	assert.Contains(t, body, `value="`+strconv.FormatInt(algebra, 10)+`">Алгебра<`)
	assert.Contains(t, body, `value="`+strconv.FormatInt(viktor, 10)+`">Кузнецов Виктор<`)
	assert.NotContains(t, body, "Ушедший Учитель")
	assert.Contains(t, body, `href="/admin/substitutions?ended=1"`)

	recorder := servertest.PostForm(t, env.Handler, substitutionsPath, substitutionForm(class, algebra, viktor, today, time.Time{}), []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, recorder, substitutionsPath)

	substitutions, err := env.School.Substitutions(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, substitutions, 1)
	assert.Equal(t, viktor, substitutions[0].TeacherID)
	assert.True(t, substitutions[0].EndDate.IsZero())
	id := substitutions[0].ID

	body = servertest.Get(t, env.Handler, substitutionsPath, admin).Body.String()
	assert.Contains(t, body, "Кузнецов Виктор")
	assert.Contains(t, body, today.Format("02.01.2006")+" — до отмены")
	assert.NotContains(t, body, "hx-confirm=")
	assert.Contains(t, body, `href="`+substitutionsPath+`?edit=`+strconv.FormatInt(id, 10)+`"`)

	overlap := servertest.PostForm(t, env.Handler, substitutionsPath, substitutionForm(class, algebra, viktor, today.AddDate(0, 0, 3), today.AddDate(0, 0, 5)), []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, overlap.Code)
	assert.Contains(t, overlap.Body.String(), "Период пересекается с другой заменой")

	reversed := servertest.PostForm(t, env.Handler, substitutionsPath, substitutionForm(class, algebra, viktor, today, today.AddDate(0, 0, -1)), []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, reversed.Code)
	assert.Contains(t, reversed.Body.String(), "Дата окончания раньше начала")

	pastClass := servertest.PostForm(t, env.Handler, substitutionsPath, substitutionForm(past, algebra, viktor, today, time.Time{}), []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, pastClass.Code)
	assert.Contains(t, pastClass.Body.String(), "Такого класса нет в текущем году")

	inactiveTeacher := servertest.PostForm(t, env.Handler, substitutionsPath, substitutionForm(class, algebra, gone, today.AddDate(0, -1, 0), today.AddDate(0, 0, -10)), []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, inactiveTeacher.Code)
	assert.Contains(t, inactiveTeacher.Body.String(), "Такого учителя нет")

	empty := servertest.PostForm(t, env.Handler, substitutionsPath, url.Values{}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, empty.Code)
	assert.Contains(t, empty.Body.String(), "Выберите класс")
	assert.Contains(t, empty.Body.String(), "Выберите предмет")
	assert.Contains(t, empty.Body.String(), "Выберите учителя")
	assert.Contains(t, empty.Body.String(), "Укажите дату начала")

	edit := servertest.Get(t, env.Handler, substitutionsPath+"?edit="+strconv.FormatInt(id, 10), admin).Body.String()
	assert.Contains(t, edit, `name="start_date" value="`+today.Format(validation.DateLayout)+`"`)
	assert.Contains(t, edit, `action="`+substitutionPath(id, "")+`"`)
	assert.Contains(t, edit, `hx-confirm="Удалить замену Кузнецов Виктор, 7А · Алгебра?"`)
	assert.Contains(t, edit, `formaction="`+substitutionPath(id, "/delete")+`"`)

	yesterday := today.AddDate(0, 0, -1)
	closed := servertest.PostForm(t, env.Handler, substitutionPath(id, ""), url.Values{
		"start_date": {yesterday.AddDate(0, 0, -7).Format(validation.DateLayout)},
		"end_date":   {yesterday.Format(validation.DateLayout)},
	}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, closed, substitutionsPath)

	body = servertest.Get(t, env.Handler, substitutionsPath, admin).Body.String()
	assert.Contains(t, body, "Пока нет замен")
	assert.NotContains(t, body, "завершена")

	body = servertest.Get(t, env.Handler, substitutionsPath+"?ended=1", admin).Body.String()
	assert.Contains(t, body, "Кузнецов Виктор")
	assert.Contains(t, body, "завершена")
	assert.NotContains(t, body, `formaction="`+substitutionPath(id, "/delete")+`?ended=1"`)
	assert.Contains(t, body, `href="`+substitutionsPath+`?edit=`+strconv.FormatInt(id, 10)+`&amp;ended=1"`)

	ended := servertest.Get(t, env.Handler, substitutionsPath+"?edit="+strconv.FormatInt(id, 10)+"&ended=1", admin).Body.String()
	assert.Contains(t, ended, `formaction="`+substitutionPath(id, "/delete")+`?ended=1"`)

	fragment := servertest.PostForm(t, env.Handler, substitutionsPath, substitutionForm(class, algebra, viktor, today, time.Time{}), []*http.Cookie{admin}, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `id="substitutions"`)
	assert.NotContains(t, fragment.Body.String(), "<html")

	removed := servertest.PostForm(t, env.Handler, substitutionPath(id, "/delete")+"?ended=1", nil, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, removed, substitutionsPath+"?ended=1")

	substitutions, err = env.School.Substitutions(t.Context(), true)
	require.NoError(t, err)
	require.Len(t, substitutions, 1)
	assert.NotEqual(t, id, substitutions[0].ID)

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, substitutionPath(id, "/delete"), nil, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, substitutionPath(id, ""), url.Values{"start_date": {"2026-09-01"}}, []*http.Cookie{admin}, nil).Code)
}

func TestSubstitutionWithLessonsCannotBeDeleted(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	ctx := t.Context()
	today := env.School.Today()

	class := createClass(t, env, admin, env.School.CurrentYear(), "7А")
	algebra := createSubject(t, env, admin, "Алгебра")
	viktor, _ := createUserVia(t, env, admin, "/admin/teachers", servertest.NameForm("Кузнецов Виктор"))

	id, err := env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: class, SubjectID: algebra, TeacherID: viktor, StartDate: today.Format(validation.DateLayout),
	})
	require.NoError(t, err)

	_, err = env.Journal.OpenLesson(ctx, viktor, env.School.CurrentYear(), today,
		journal.OpenLessonInput{ClassID: class, SubjectID: algebra, Date: today.Format(validation.DateLayout)})
	require.NoError(t, err)

	refused := servertest.PostForm(t, env.Handler, substitutionPath(id, "/delete"), nil, []*http.Cookie{admin}, nil)
	require.Equal(t, http.StatusOK, refused.Code)
	assert.Contains(t, refused.Body.String(), "По замене уже проведены уроки: вместо удаления укажите дату окончания")
	assert.Contains(t, refused.Body.String(), `action="`+substitutionPath(id, "")+`"`)

	ended := servertest.PostForm(t, env.Handler, substitutionPath(id, ""),
		url.Values{"start_date": {today.Format(validation.DateLayout)}, "end_date": {today.Format(validation.DateLayout)}},
		[]*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, ended, substitutionsPath)
}

func TestSubstitutionsHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.LoginAs(t, "teacher")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, substitutionsPath, teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, substitutionsPath, url.Values{}, []*http.Cookie{teacher}, nil).Code)
	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, substitutionsPath), "/login")
}
