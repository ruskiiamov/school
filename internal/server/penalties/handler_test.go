package penalties_test

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
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

type fixture struct {
	env      *servertest.Env
	teacher  int64
	other    int64
	class    int64
	subject  int64
	maria    int64
	petr     int64
	lesson   int64
	homework int64
	year     string
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	ctx := t.Context()
	env := servertest.New(t)
	env.EnablePenalties(t)

	class, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)

	subject, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	f := fixture{
		env:     env,
		teacher: createUser(t, env, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна"),
		other:   createUser(t, env, auth.RoleTeacher, "other", "Кузнецов Виктор Павлович"),
		maria:   createUser(t, env, auth.RoleStudent, "maria", "Иванова Мария Петровна"),
		petr:    createUser(t, env, auth.RoleStudent, "petr", "Козлов Пётр Ильич"),
		year:    strconv.Itoa(env.School.CurrentYear()),
	}

	f.class, f.subject = class, subject
	require.NoError(t, env.School.AssignTeacher(ctx, class, subject, f.teacher))

	for _, id := range []int64{f.maria, f.petr} {
		require.NoError(t, env.School.AddClassStudent(ctx, class, id))
	}

	f.homework, err = env.Journal.CreateWorkType(ctx, f.teacher, subject, "Домашняя работа", true)
	require.NoError(t, err)

	f.lesson = env.CreateLesson(t, f.teacher, class, subject, env.School.Today())

	return f
}

func createUser(t *testing.T, env *servertest.Env, role auth.Role, login, fullName string) int64 {
	t.Helper()

	env.CreateUser(t, role, login, fullName)

	users, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: role})
	require.NoError(t, err)

	for _, user := range users {
		if user.Login == login {
			return user.ID
		}
	}

	t.Fatalf("user %q not found", login)

	return 0
}

func studentPath(id int64, suffix string) string {
	return "/penalties/students/" + strconv.FormatInt(id, 10) + suffix
}

func workoffForm(date, points, comment string) url.Values {
	return url.Values{"date": {date}, "points": {points}, "comment": {comment}}
}

func TestDebtorsList(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "30")
	teacher := f.env.LoginAs(t, "teacher")

	recorder := servertest.Get(t, f.env.Handler, "/penalties", teacher)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "Мытьё полов")
	assert.Contains(t, body, "Откройте ученика, чтобы записать отработку")
	assert.Contains(t, body, "Иванова Мария Петровна")
	assert.Contains(t, body, "Долг: 14 м²")
	assert.NotContains(t, body, "Козлов Пётр Ильич")
	assert.Contains(t, body, `href="`+studentPath(f.maria, "?year="+f.year)+`"`)

	body = servertest.Get(t, f.env.Handler, "/penalties?q=Козлов", teacher).Body.String()
	assert.Contains(t, body, "Никого не нашли")

	f.env.AddWorkoff(t, f.teacher, f.maria, "14")

	body = servertest.Get(t, f.env.Handler, "/penalties", teacher).Body.String()
	assert.Contains(t, body, "Долгов нет")
}

func TestWorkoffRecorded(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "0")
	teacher := f.env.LoginAs(t, "teacher")
	today := f.env.School.Today().Format(validation.InputDateLayout)

	page := servertest.Get(t, f.env.Handler, studentPath(f.maria, ""), teacher).Body.String()
	assert.Contains(t, page, "Иванова Мария Петровна · 7А")
	assert.Contains(t, page, "Долг: 20 м²")
	assert.Contains(t, page, `value="`+today+`"`)
	assert.Contains(t, page, "не погашено")

	recorder := servertest.PostForm(t, f.env.Handler, studentPath(f.maria, "/workoffs?year="+f.year),
		workoffForm(today, "16", "коридор 2 этажа"), []*http.Cookie{teacher}, nil)
	servertest.AssertRedirect(t, recorder, studentPath(f.maria, "?year="+f.year))

	page = servertest.Get(t, f.env.Handler, studentPath(f.maria, ""), teacher).Body.String()
	assert.Contains(t, page, "Долг: 4 м²")
	assert.Contains(t, page, "16 м²")
	assert.Contains(t, page, "Сидорова А. А.")
	assert.Contains(t, page, "коридор 2 этажа")
	assert.Contains(t, page, "частично")

	recorder = servertest.PostForm(t, f.env.Handler, studentPath(f.maria, "/workoffs"),
		workoffForm(today, "4", ""), []*http.Cookie{teacher}, map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Долга нет")
	assert.Contains(t, recorder.Body.String(), "погашено")
}

func TestWorkoffErrors(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "10")
	teacher := f.env.LoginAs(t, "teacher")
	today := f.env.School.Today()
	todayValue := today.Format(validation.InputDateLayout)

	tests := []struct {
		name    string
		form    url.Values
		message string
	}{
		{"future", workoffForm(today.AddDate(0, 0, 1).Format(validation.InputDateLayout), "5", ""), "Дата не может быть позже сегодняшней"},
		{"no date", workoffForm("", "5", ""), "Укажите дату"},
		{"empty", workoffForm(todayValue, "", ""), "Отработано: целое число больше 0"},
		{"zero", workoffForm(todayValue, "0", ""), "Отработано: целое число больше 0"},
		{"negative", workoffForm(todayValue, "-3", ""), "Отработано: целое число больше 0"},
		{"fraction", workoffForm(todayValue, "2,5", ""), "Отработано: целое число больше 0"},
		{"over debt", workoffForm(todayValue, "19", ""), "Больше долга: долг 18"},
		{"long comment", workoffForm(todayValue, "5", strings.Repeat("а", 201)), "Комментарий: не больше 200 символов"},
	}

	for _, tt := range tests {
		recorder := servertest.PostForm(t, f.env.Handler, studentPath(f.maria, "/workoffs"), tt.form, []*http.Cookie{teacher}, nil)
		require.Equal(t, http.StatusOK, recorder.Code, tt.name)
		assert.Contains(t, recorder.Body.String(), tt.message, tt.name)
	}

	debt, err := f.env.Journal.StudentDebt(t.Context(), f.maria)
	require.NoError(t, err)
	assert.Equal(t, 18, debt)
}

func TestWorkoffCoversOlderLessonFirst(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	start, _ := f.env.School.YearBounds(f.env.School.CurrentYear())
	class, err := f.env.School.CreateClass(ctx, "8Б")
	require.NoError(t, err)
	require.NoError(t, f.env.School.AssignTeacher(ctx, class, f.subject, f.teacher))
	require.NoError(t, f.env.School.RemoveClassStudent(ctx, f.class, f.maria))
	require.NoError(t, f.env.School.AddClassStudent(ctx, class, f.maria))

	older := f.env.CreateLesson(t, f.teacher, class, f.subject, start)
	newer := f.env.CreateLesson(t, f.teacher, class, f.subject, start.AddDate(0, 0, 1))

	f.env.SetPenalty(t, f.teacher, newer, f.maria, f.homework, "50")
	f.env.SetPenalty(t, f.teacher, older, f.maria, f.homework, "25")
	f.env.AddWorkoff(t, f.teacher, f.maria, "15")

	history, err := f.env.Journal.PenaltyHistory(ctx, f.maria)
	require.NoError(t, err)
	require.Len(t, history.Penalties, 2)

	assert.Equal(t, newer, history.Penalties[0].LessonID)
	assert.Equal(t, journal.PenaltyOpen, history.Penalties[0].Status)
	assert.Equal(t, older, history.Penalties[1].LessonID)
	assert.Equal(t, journal.PenaltyPaid, history.Penalties[1].Status)
	assert.Equal(t, 10, history.Debt)
}

func TestHistorySameDayNewestFirst(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	classwork, err := f.env.Journal.CreateWorkType(t.Context(), f.teacher, f.subject, "Классная работа", true)
	require.NoError(t, err)

	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, classwork, "0")
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "50")
	f.env.AddWorkoff(t, f.teacher, f.maria, "5")

	history, err := f.env.Journal.PenaltyHistory(t.Context(), f.maria)
	require.NoError(t, err)
	require.Len(t, history.Penalties, 2)

	assert.Equal(t, "Домашняя работа", history.Penalties[0].WorkTypeName)
	assert.Equal(t, journal.PenaltyOpen, history.Penalties[0].Status)
	assert.Equal(t, "Классная работа", history.Penalties[1].WorkTypeName)
	assert.Equal(t, journal.PenaltyPartial, history.Penalties[1].Status)
}

func TestWorkoffDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "0")
	f.env.AddWorkoff(t, f.teacher, f.maria, "5")

	history, err := f.env.Journal.PenaltyHistory(t.Context(), f.maria)
	require.NoError(t, err)
	require.Len(t, history.Workoffs, 1)

	deletePath := "/penalties/workoffs/" + strconv.FormatInt(history.Workoffs[0].ID, 10) + "/delete"
	other := f.env.LoginAs(t, "other")

	page := servertest.Get(t, f.env.Handler, studentPath(f.maria, ""), other).Body.String()
	assert.NotContains(t, page, deletePath)

	recorder := servertest.PostForm(t, f.env.Handler, deletePath, url.Values{}, []*http.Cookie{other}, nil)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	teacher := f.env.LoginAs(t, "teacher")
	page = servertest.Get(t, f.env.Handler, studentPath(f.maria, ""), teacher).Body.String()
	assert.Contains(t, page, deletePath)
	assert.Contains(t, page, "Удалить отработку от ")

	admin := servertest.Login(t, f.env.Handler)
	recorder = servertest.PostForm(t, f.env.Handler, deletePath+"?year="+f.year, url.Values{}, []*http.Cookie{admin}, nil)
	servertest.AssertRedirect(t, recorder, studentPath(f.maria, "?year="+f.year))

	debt, err := f.env.Journal.StudentDebt(t.Context(), f.maria)
	require.NoError(t, err)
	assert.Equal(t, 20, debt)
}

func TestPenaltiesAccess(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.env.CreateUser(t, auth.RoleParent, "parent", "Иванов Пётр Сергеевич")

	for _, login := range []string{"maria", "parent"} {
		cookie := f.env.LoginAs(t, login)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/penalties", cookie).Code, login)
		assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, studentPath(f.maria, ""), cookie).Code, login)
	}

	teacher := f.env.LoginAs(t, "teacher")
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, studentPath(f.teacher, ""), teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, studentPath(999, ""), teacher).Code)

	f.env.DisablePenalties(t)

	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, "/penalties", teacher).Code)
	assert.Equal(t, http.StatusNotFound, servertest.Get(t, f.env.Handler, studentPath(f.maria, ""), teacher).Code)
	assert.NotContains(t, servertest.Get(t, f.env.Handler, "/", teacher).Body.String(), `href="/penalties"`)
}
