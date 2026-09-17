package admin_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func createSubject(t *testing.T, env *servertest.Env, admin *http.Cookie, name string) int64 {
	t.Helper()

	recorder := servertest.PostForm(t, env.Handler, "/admin/subjects", url.Values{"name": {name}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, recorder, "/admin/subjects")

	subjects, err := env.School.Subjects(t.Context(), true)
	require.NoError(t, err)

	for _, subject := range subjects {
		if subject.Name == name && subject.Active {
			return subject.ID
		}
	}

	t.Fatalf("subject %q not found", name)

	return 0
}

func subjectPath(id int64, suffix string) string {
	return "/admin/subjects/" + strconv.FormatInt(id, 10) + suffix
}

func TestSubjectsEmptyListAndCreate(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	empty := servertest.Get(t, env.Handler, "/admin/subjects", admin)
	assert.Equal(t, http.StatusOK, empty.Code)
	assert.Contains(t, empty.Body.String(), "Пока нет предметов")
	assert.Contains(t, empty.Body.String(), `action="/admin/subjects"`)
	assert.Contains(t, empty.Body.String(), `hx-target="#subjects"`)

	messy := servertest.PostForm(t, env.Handler, "/admin/subjects", url.Values{"name": {"  Алгебра   и начала  анализа "}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, messy, "/admin/subjects")

	subjects, err := env.School.Subjects(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, subjects, 1)
	assert.Equal(t, "Алгебра и начала анализа", subjects[0].Name)

	list := servertest.Get(t, env.Handler, "/admin/subjects", admin).Body.String()
	assert.Contains(t, list, ">Алгебра и начала анализа</span>")
	assert.NotContains(t, list, `value="Алгебра и начала анализа"`)
	assert.Contains(t, list, `href="/admin/subjects?edit=`+strconv.FormatInt(subjects[0].ID, 10)+`"`)
	assert.NotContains(t, list, `formaction="`+subjectPath(subjects[0].ID, "/deactivate")+`"`)
	assert.NotContains(t, list, `aria-label="Изменить"`)
	assert.NotContains(t, list, `aria-label="Удалить"`)
	assert.NotContains(t, list, "Пока нет предметов")
}

func TestSubjectEditModeShowsFormForOneRow(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	algebra := createSubject(t, env, admin, "Алгебра")
	history := createSubject(t, env, admin, "История")

	body := servertest.Get(t, env.Handler, "/admin/subjects?edit="+strconv.FormatInt(algebra, 10), admin).Body.String()
	assert.Contains(t, body, `value="Алгебра"`)
	assert.Contains(t, body, `action="`+subjectPath(algebra, "")+`"`)
	assert.NotContains(t, body, `aria-label="Отмена"`)
	assert.Contains(t, body, `aria-label="Сохранить"`)
	assert.Contains(t, body, `aria-label="Удалить"`)
	assert.Contains(t, body, "data-edit-form")
	assert.Contains(t, body, `data-cancel="/admin/subjects"`)
	assert.Contains(t, body, `formaction="`+subjectPath(algebra, "/deactivate")+`"`)
	assert.NotContains(t, body, `value="История"`)
	assert.NotContains(t, body, `formaction="`+subjectPath(history, "/deactivate")+`"`)
	assert.Contains(t, body, `href="/admin/subjects?edit=`+strconv.FormatInt(history, 10)+`"`)

	fragment := servertest.Get(t, env.Handler, "/admin/subjects?edit="+strconv.FormatInt(algebra, 10)+"&inactive=1", admin, map[string]string{"HX-Request": "true"})
	assert.Equal(t, http.StatusOK, fragment.Code)
	assert.NotContains(t, fragment.Body.String(), "<html")
	assert.Contains(t, fragment.Body.String(), `action="`+subjectPath(algebra, "?inactive=1")+`"`)
	assert.Contains(t, fragment.Body.String(), `data-cancel="/admin/subjects?inactive=1"`)

	assert.NotContains(t, servertest.Get(t, env.Handler, "/admin/subjects?edit=abc", admin).Body.String(), `aria-label="Название предмета"`)
}

func TestSubjectsAreSortedByName(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	createSubject(t, env, admin, "История")
	createSubject(t, env, admin, "Алгебра")

	body := servertest.Get(t, env.Handler, "/admin/subjects", admin).Body.String()
	assert.Less(t, strings.Index(body, "Алгебра"), strings.Index(body, "История"))
}

func TestSubjectRenameAndValidation(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	id := createSubject(t, env, admin, "Алгебра")
	createSubject(t, env, admin, "Геометрия")

	renamed := servertest.PostForm(t, env.Handler, subjectPath(id, ""), url.Values{"name": {"Математика"}}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, renamed, "/admin/subjects")

	subject, err := env.School.SubjectByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "Математика", subject.Name)

	taken := servertest.PostForm(t, env.Handler, subjectPath(id, ""), url.Values{"name": {"Геометрия"}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, taken.Code)
	assert.Contains(t, taken.Body.String(), "<html")
	assert.Contains(t, taken.Body.String(), "уже есть среди активных")
	assert.Equal(t, 1, strings.Count(taken.Body.String(), `value="Геометрия"`))
	assert.Contains(t, taken.Body.String(), ">Геометрия</span>")
	assert.NotContains(t, taken.Body.String(), "Математика")

	blank := servertest.PostForm(t, env.Handler, "/admin/subjects", url.Values{"name": {"   "}}, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, blank.Code)
	assert.Contains(t, blank.Body.String(), "Укажите название")

	subject, err = env.School.SubjectByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "Математика", subject.Name)
}

func TestSubjectDeactivateHidesAndFreesName(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	id := createSubject(t, env, admin, "Алгебра")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, subjectPath(id, "/deactivate?inactive=1"), nil, []*http.Cookie{admin}, nil), "/admin/subjects?inactive=1")

	visible := servertest.Get(t, env.Handler, "/admin/subjects", admin).Body.String()
	assert.NotContains(t, visible, "Алгебра")
	assert.Contains(t, visible, "Показать удалённые")

	all := servertest.Get(t, env.Handler, "/admin/subjects?inactive=1", admin).Body.String()
	assert.Contains(t, all, "Алгебра")
	assert.Contains(t, all, "удалён")
	assert.Contains(t, all, "Скрыть удалённые")
	assert.NotContains(t, all, `formaction="`+subjectPath(id, "/activate?inactive=1")+`"`)
	assert.Contains(t, all, `action="/admin/subjects?inactive=1"`)
	assert.Contains(t, all, `href="/admin/subjects?edit=`+strconv.FormatInt(id, 10)+`&amp;inactive=1"`)

	editing := servertest.Get(t, env.Handler, "/admin/subjects?edit="+strconv.FormatInt(id, 10)+"&inactive=1", admin).Body.String()
	assert.Contains(t, editing, `value="Алгебра"`)
	assert.Contains(t, editing, `formaction="`+subjectPath(id, "/activate?inactive=1")+`"`)
	assert.Contains(t, editing, `aria-label="Восстановить"`)
	assert.NotContains(t, editing, "required")

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, subjectPath(id, "?inactive=1"), url.Values{"name": {"Алгебра (старая)"}}, []*http.Cookie{admin}, nil), "/admin/subjects?inactive=1")

	renamed, err := env.School.SubjectByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "Алгебра (старая)", renamed.Name)
	assert.False(t, renamed.Active)

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, subjectPath(id, "?inactive=1"), url.Values{"name": {"Алгебра"}}, []*http.Cookie{admin}, nil), "/admin/subjects?inactive=1")

	replacement := createSubject(t, env, admin, "Алгебра")
	assert.NotEqual(t, id, replacement)

	conflict := servertest.PostForm(t, env.Handler, subjectPath(id, "/activate?inactive=1"), nil, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "уже есть среди активных")
	assert.Equal(t, 1, strings.Count(conflict.Body.String(), `value="Алгебра"`))
	assert.Contains(t, conflict.Body.String(), `formaction="`+subjectPath(id, "/activate?inactive=1")+`"`)

	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, subjectPath(replacement, "/deactivate"), nil, []*http.Cookie{admin}, nil), "/admin/subjects")
	servertest.AssertRedirect(t, servertest.PostForm(t, env.Handler, subjectPath(id, "/activate"), nil, []*http.Cookie{admin}, nil), "/admin/subjects")

	subject, err := env.School.SubjectByID(t.Context(), id)
	require.NoError(t, err)
	assert.True(t, subject.Active)
}

func TestSubjectUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/subjects/999", url.Values{"name": {"X"}}, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/subjects/abc", url.Values{"name": {"X"}}, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/subjects/999/deactivate", nil, []*http.Cookie{admin}, nil).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/subjects/999/edit", admin).Code)

	_, err := env.School.SubjectByID(t.Context(), 999)
	assert.ErrorIs(t, err, school.ErrNotFound)
}

func TestSubjectsHiddenFromOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.LoginAs(t, "teacher")

	servertest.AssertRedirect(t, servertest.Get(t, env.Handler, "/admin/subjects"), "/login")
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, "/admin/subjects", teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, "/admin/subjects", url.Values{"name": {"Алгебра"}}, []*http.Cookie{teacher}, nil).Code)

	subjects, err := env.School.Subjects(t.Context(), true)
	require.NoError(t, err)
	assert.Empty(t, subjects)
}

func TestSubjectHtmxRequestsGetListFragment(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	htmx := map[string]string{"HX-Request": "true"}

	created := servertest.PostForm(t, env.Handler, "/admin/subjects", url.Values{"name": {"Алгебра"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, created.Code)
	assert.NotContains(t, created.Body.String(), "<html")
	assert.Contains(t, created.Body.String(), `id="subjects"`)
	assert.Contains(t, created.Body.String(), ">Алгебра</span>")

	duplicate := servertest.PostForm(t, env.Handler, "/admin/subjects", url.Values{"name": {"Алгебра"}}, []*http.Cookie{admin}, htmx)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.NotContains(t, duplicate.Body.String(), "<html")
	assert.Contains(t, duplicate.Body.String(), "уже есть среди активных")
}
