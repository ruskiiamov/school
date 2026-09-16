package admin_test

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
	"github.com/ruskiiamov/school/internal/storage"
)

const transferURL = "/admin/classes/transfer"

type transferFixture struct {
	env      *servertest.Env
	admin    *http.Cookie
	past     int
	current  int
	seventh  int64
	eleventh int64
	ivanov   int64
	petrova  int64
	sidorov  int64
}

func newTransferFixture(t *testing.T) transferFixture {
	t.Helper()

	env := servertest.New(t)
	f := transferFixture{env: env, admin: servertest.Login(t, env.Handler), current: env.School.CurrentYear()}
	f.past = f.current - 1

	f.seventh = createPastClass(t, env, f.past, "7А")
	f.eleventh = createPastClass(t, env, f.past, "11А")

	env.CreateUser(t, auth.RoleStudent, "ivanov", "Иванов Пётр")
	env.CreateUser(t, auth.RoleStudent, "petrova", "Петрова Анна")
	env.CreateUser(t, auth.RoleStudent, "sidorov", "Сидоров Олег")
	f.ivanov = userIDByLogin(t, env, "ivanov")
	f.petrova = userIDByLogin(t, env, "petrova")
	f.sidorov = userIDByLogin(t, env, "sidorov")

	members := storage.NewClassStudentRepo(env.DB)
	require.NoError(t, members.Add(t.Context(), f.seventh, f.ivanov))
	require.NoError(t, members.Add(t.Context(), f.seventh, f.petrova))
	require.NoError(t, members.Add(t.Context(), f.eleventh, f.sidorov))
	require.NoError(t, env.Auth.SetUserActive(t.Context(), f.petrova, false))

	return f
}

func (f transferFixture) key(classID int64) string {
	return strconv.FormatInt(classID, 10)
}

func TestTransferPageProposesNextNames(t *testing.T) {
	t.Parallel()

	f := newTransferFixture(t)

	body := servertest.Get(t, f.env.Handler, transferURL, f.admin).Body.String()
	assert.Contains(t, body, school.YearName(f.past)+" → "+school.YearName(f.current))
	assert.Contains(t, body, `name="name-`+f.key(f.seventh)+`" value="8А"`)
	assert.Contains(t, body, `name="transfer-`+f.key(f.seventh)+`" value="1" checked`)
	assert.NotContains(t, body, `name="transfer-`+f.key(f.eleventh)+`" value="1" checked`)
	assert.NotContains(t, body, "graduating-")
	assert.Contains(t, body, ">выпускной<")
	assert.Contains(t, body, `name="students-`+f.key(f.seventh)+`" value="`+f.key(f.ivanov)+`" checked`)
	assert.NotContains(t, body, `name="students-`+f.key(f.seventh)+`" value="`+f.key(f.petrova)+`"`)
	assert.Contains(t, body, "Петрова Анна")
	assert.NotContains(t, body, "required")

	classes := servertest.Get(t, f.env.Handler, "/admin/classes", f.admin).Body.String()
	assert.Contains(t, classes, `href="`+transferURL+`"`)

	past := servertest.Get(t, f.env.Handler, classesURL(f.past, ""), f.admin).Body.String()
	assert.NotContains(t, past, `href="`+transferURL+`"`)
}

func TestTransferCreatesClasses(t *testing.T) {
	t.Parallel()

	f := newTransferFixture(t)

	form := url.Values{
		"transfer-" + f.key(f.seventh): {"1"},
		"name-" + f.key(f.seventh):     {" 8А "},
		"students-" + f.key(f.seventh): {f.key(f.ivanov)},
		"name-" + f.key(f.eleventh):    {"12А"},
	}

	recorder := servertest.PostForm(t, f.env.Handler, transferURL, form, []*http.Cookie{f.admin}, nil)
	servertest.AssertRedirect(t, recorder, classesURL(f.current, ""))

	classes, err := f.env.School.Classes(t.Context(), f.current, true)
	require.NoError(t, err)
	require.Len(t, classes, 1)
	assert.Equal(t, "8А", classes[0].Name)

	ids, err := storage.NewClassStudentRepo(f.env.DB).StudentIDs(t.Context(), classes[0].ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.ivanov}, ids)

	again := servertest.Get(t, f.env.Handler, transferURL, f.admin).Body.String()
	assert.Contains(t, again, "Такой класс уже есть")
	assert.Contains(t, again, ">11А</h2>")
	assert.NotContains(t, again, `name="transfer-`+f.key(f.eleventh)+`" value="1" checked`)
	assert.NotContains(t, again, `name="transfer-`+f.key(f.seventh)+`" value="1" checked`)
}

func TestTransferErrors(t *testing.T) {
	t.Parallel()

	f := newTransferFixture(t)
	createClass(t, f.env, f.admin, f.current, "8Б")

	tests := []struct {
		name    string
		form    url.Values
		message string
	}{
		{
			name:    "taken name",
			form:    url.Values{"transfer-" + f.key(f.seventh): {"1"}, "name-" + f.key(f.seventh): {"8Б"}},
			message: "Такой класс в этом году уже есть",
		},
		{
			name:    "empty name",
			form:    url.Values{"transfer-" + f.key(f.seventh): {"1"}, "name-" + f.key(f.seventh): {""}},
			message: "Укажите название",
		},
		{
			name: "inactive student",
			form: url.Values{
				"transfer-" + f.key(f.seventh): {"1"},
				"name-" + f.key(f.seventh):     {"8А"},
				"students-" + f.key(f.seventh): {f.key(f.petrova)},
			},
			message: "Ученик не может быть переведён",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := servertest.PostForm(t, f.env.Handler, transferURL, tt.form, []*http.Cookie{f.admin}, nil)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), tt.message)

			classes, err := f.env.School.Classes(t.Context(), f.current, true)
			require.NoError(t, err)
			assert.Len(t, classes, 1)
		})
	}
}

func TestTransferNothingSelected(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	seventh := createPastClass(t, env, env.School.CurrentYear()-1, "7А")

	form := url.Values{"name-" + strconv.FormatInt(seventh, 10): {"8А"}}
	recorder := servertest.PostForm(t, env.Handler, transferURL, form, []*http.Cookie{admin}, nil)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Выберите хотя бы один класс")

	classes, err := env.School.Classes(t.Context(), env.School.CurrentYear(), true)
	require.NoError(t, err)
	assert.Empty(t, classes)
}

func TestTransferForbiddenForOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	teacher := env.LoginAs(t, "teacher")

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, env.Handler, transferURL, teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.PostForm(t, env.Handler, transferURL, url.Values{}, []*http.Cookie{teacher}, nil).Code)

	recorder := servertest.Get(t, env.Handler, transferURL)
	servertest.AssertRedirect(t, recorder, "/login")
}

func TestTransferButtonHiddenWithoutPastClasses(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)

	body := servertest.Get(t, env.Handler, "/admin/classes", admin).Body.String()
	assert.NotContains(t, body, `href="`+transferURL+`"`)

	page := servertest.Get(t, env.Handler, transferURL, admin).Body.String()
	assert.Contains(t, page, "нет классов для перевода")
}
