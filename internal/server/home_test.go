package server_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
)

func TestHomeShowsSetupStepsForAdmin(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	ctx := t.Context()

	body := servertest.Get(t, env.Handler, "/", admin).Body.String()
	yearName := school.YearName(env.School.CurrentYear())

	assert.Contains(t, body, "Начало работы")
	assert.Contains(t, body, "Кто что ведёт")
	assert.Equal(t, 6, strings.Count(body, "Не сделано"))
	assert.Contains(t, body, "Учебный год "+yearName)

	_, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	body = servertest.Get(t, env.Handler, "/", admin).Body.String()
	assert.Equal(t, 5, strings.Count(body, "Не сделано"))
	assert.Equal(t, 1, strings.Count(body, "Готово"))
}

func TestHomeHidesSetupWhenRequiredStepsDone(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	ctx := t.Context()

	classID, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)
	subjectID, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)
	teacher, err := env.Auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleTeacher, Name: auth.Name{Last: "Сидорова", First: "Анна"}})
	require.NoError(t, err)
	student, err := env.Auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleStudent, Name: auth.Name{Last: "Козлов", First: "Пётр"}})
	require.NoError(t, err)
	require.NoError(t, env.School.SetStudentClass(ctx, student.User.ID, classID))

	body := servertest.Get(t, env.Handler, "/", admin).Body.String()
	assert.Contains(t, body, "Начало работы")

	require.NoError(t, env.School.AssignTeacher(ctx, classID, subjectID, teacher.User.ID))

	body = servertest.Get(t, env.Handler, "/", admin).Body.String()
	assert.NotContains(t, body, "Начало работы")
}

func TestHomeCountsCurrentYearForAdmin(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	admin := servertest.Login(t, env.Handler)
	ctx := t.Context()

	classID, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)

	_, err = env.School.CreateClass(ctx, "8Б")
	require.NoError(t, err)

	deleted, err := env.School.CreateClass(ctx, "9В")
	require.NoError(t, err)
	require.NoError(t, env.School.SetClassActive(ctx, deleted, false))

	_, err = env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)

	inactiveSubject, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Черчение"})
	require.NoError(t, err)
	require.NoError(t, env.School.SetSubjectActive(ctx, inactiveSubject, false))

	for _, name := range []string{"Сидорова Анна", "Петров Иван", "Уволенный Учитель"} {
		created, err := env.Auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleTeacher, Name: servertest.Name(name)})
		require.NoError(t, err)

		if name == "Уволенный Учитель" {
			require.NoError(t, env.Auth.SetUserActive(ctx, created.User.ID, false))
		}
	}

	env.CreateUser(t, auth.RoleStudent, "free", "Без Класса")

	for _, name := range []string{"Козлов Пётр", "Иванова Мария", "Выбывший Ученик"} {
		created, err := env.Auth.CreateUser(ctx, auth.NewUser{Role: auth.RoleStudent, Name: servertest.Name(name)})
		require.NoError(t, err)
		require.NoError(t, env.School.SetStudentClass(ctx, created.User.ID, classID))

		if name == "Выбывший Ученик" {
			require.NoError(t, env.Auth.SetUserActive(ctx, created.User.ID, false))
		}
	}

	body := servertest.Get(t, env.Handler, "/", admin).Body.String()

	assert.Contains(t, body, "Начало работы")
	assertStat(t, body, "Классы", 2)
	assertStat(t, body, "Ученики", 2)
	assertStat(t, body, "Учителя", 2)
	assertStat(t, body, "Предметы", 1)
	assert.Contains(t, body, `href="/admin/students"`)
}

func assertStat(t *testing.T, body, label string, value int) {
	t.Helper()

	assert.Contains(t, body, label+`</span></span> <span class="mt-2 block text-2xl font-semibold tabular-nums text-slate-900 lg:text-3xl">`+strconv.Itoa(value)+`</span>`, label)
}

func TestHomeTeacherSeesPairsOfYear(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	ctx := t.Context()
	year := env.School.CurrentYear()
	today := env.School.Today()
	teacherID := createUser(t, env, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	otherID := createUser(t, env, auth.RoleTeacher, "other", "Кузнецов Виктор Павлович")
	teacher := env.LoginAs(t, "teacher")

	body := servertest.Get(t, env.Handler, "/", teacher).Body.String()
	assert.Contains(t, body, "В этом году вам не назначены классы и предметы")
	assert.Contains(t, body, `name="year"`)
	assert.Contains(t, body, `<noscript><button type="submit"`)
	assert.NotContains(t, body, `href="/journal"`)
	assert.NotContains(t, body, "OLD")
	assert.NotContains(t, body, "Урок сегодня")

	seventh, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)
	eighth, err := env.School.CreateClass(ctx, "8А")
	require.NoError(t, err)
	ninth, err := env.School.CreateClass(ctx, "9Б")
	require.NoError(t, err)
	algebra, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)
	geometry, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Геометрия"})
	require.NoError(t, err)

	require.NoError(t, env.School.AssignTeacher(ctx, seventh, algebra, teacherID))
	require.NoError(t, env.School.AssignTeacher(ctx, eighth, geometry, otherID))
	require.NoError(t, env.School.AssignTeacher(ctx, ninth, algebra, teacherID))

	start := today.AddDate(0, 0, -2)
	_, err = env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: eighth, SubjectID: geometry, TeacherID: teacherID,
		StartDate: start.Format(validation.DateLayout), EndDate: today.Format(validation.DateLayout),
	})
	require.NoError(t, err)

	open := func(classID, subjectID int64, date time.Time) {
		t.Helper()
		env.CreateLesson(t, teacherID, classID, subjectID, date)
	}

	open(seventh, algebra, today.AddDate(0, 0, -7))
	open(seventh, algebra, today)
	open(eighth, geometry, today.AddDate(0, 0, -1))
	open(ninth, algebra, today.AddDate(0, 0, -3))
	require.NoError(t, env.School.AssignTeacher(ctx, ninth, algebra, otherID))

	pairHref := func(classID, subjectID int64) string {
		return `href="/journal/class/` + strconv.FormatInt(classID, 10) + "/subject/" + strconv.FormatInt(subjectID, 10) + `"`
	}

	body = servertest.Get(t, env.Handler, "/", teacher).Body.String()
	assert.NotContains(t, body, "В этом году вам не назначены классы и предметы")
	assert.Less(t, strings.Index(body, ">Мои классы<"), strings.Index(body, ">7А · Алгебра<"))
	assert.Less(t, strings.Index(body, ">7А · Алгебра<"), strings.Index(body, ">Замены<"))
	assert.Less(t, strings.Index(body, ">Замены<"), strings.Index(body, ">8А · Геометрия<"))
	assert.Less(t, strings.Index(body, ">8А · Геометрия<"), strings.Index(body, ">9Б · Алгебра<"))
	assert.Contains(t, body, pairHref(seventh, algebra))
	assert.Contains(t, body, pairHref(eighth, geometry))
	assert.Contains(t, body, pairHref(ninth, algebra))
	assert.Contains(t, body, ">2 урока<")
	assert.Contains(t, body, view.FormatPeriod(start, today))
	assert.Equal(t, 2, strings.Count(body, ">1 урок<"))
	assert.Contains(t, body, ">только ваши уроки<")
	assert.NotContains(t, body, view.FormatShortDate(today.AddDate(0, 0, -3)))

	fragment := servertest.Get(t, env.Handler, "/?year="+strconv.Itoa(year+1), teacher, map[string]string{"HX-Request": "true"})
	require.Equal(t, http.StatusOK, fragment.Code)
	assert.Contains(t, fragment.Body.String(), `id="teacher-home"`)
	assert.Contains(t, fragment.Body.String(), "В этом году вам не назначены классы и предметы")
	assert.NotContains(t, fragment.Body.String(), "<html")

	other := servertest.Get(t, env.Handler, "/", env.LoginAs(t, "other")).Body.String()
	assert.Contains(t, other, pairHref(eighth, geometry))
	assert.Contains(t, other, pairHref(ninth, algebra))
	assert.NotContains(t, other, pairHref(seventh, algebra))
	assert.NotContains(t, other, ">Замены<")

	_, err = env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: ninth, SubjectID: algebra, TeacherID: teacherID, StartDate: today.Format(validation.DateLayout),
	})
	require.NoError(t, err)

	body = servertest.Get(t, env.Handler, "/", teacher).Body.String()
	assert.NotContains(t, body, ">только ваши уроки<")
	assert.Contains(t, body, view.FormatPeriod(today, time.Time{}))
	assert.Contains(t, body, "Уроков пока нет")
}

func TestHomeStudentSeesTodayAndWeekMarks(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	ctx := t.Context()
	teacherID := createUser(t, env, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	studentID := createUser(t, env, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	parentID := createUser(t, env, auth.RoleParent, "parent", "Козлова Ольга Николаевна")
	createUser(t, env, auth.RoleParent, "childless", "Бездетный Родитель Иванович")

	student := env.LoginAs(t, "student")

	body := servertest.Get(t, env.Handler, "/", student).Body.String()
	assert.Contains(t, body, "Уроков сегодня пока нет")
	assert.Contains(t, body, "За последние семь дней оценок нет")

	classID, err := env.School.CreateClass(ctx, "7А")
	require.NoError(t, err)
	subjectID, err := env.School.CreateSubject(ctx, school.SubjectInput{Name: "Алгебра"})
	require.NoError(t, err)
	require.NoError(t, env.School.AddClassStudent(ctx, classID, studentID))
	require.NoError(t, env.School.AssignTeacher(ctx, classID, subjectID, teacherID))
	require.NoError(t, env.School.AddChild(ctx, parentID, studentID))

	workTypes := env.EnsureWorkTypes(t, teacherID, subjectID)

	today := env.School.Today()
	lessonID := env.CreateLesson(t, teacherID, classID, subjectID, today)
	env.SetMark(t, teacherID, lessonID, studentID, workTypes[0].ID, "5")

	lessonHref := "/diary?date=" + today.Format(validation.DateLayout) + "&amp;lesson=" + strconv.FormatInt(lessonID, 10)

	body = servertest.Get(t, env.Handler, "/", student).Body.String()
	assert.Contains(t, body, ">Сегодня</h2>")
	assert.Contains(t, body, "Алгебра")
	assert.Contains(t, body, `href="`+lessonHref+`"`)
	assert.Contains(t, body, "Оценки за неделю")
	assert.Contains(t, body, "Средний: 5,00")
	assert.NotContains(t, body, `aria-label="Дети"`)

	parent := env.LoginAs(t, "parent")
	body = servertest.Get(t, env.Handler, "/", parent).Body.String()
	assert.NotContains(t, body, `aria-label="Дети"`)
	assert.Contains(t, body, "child="+strconv.FormatInt(studentID, 10))
	assert.Contains(t, body, "Средний: 5,00")

	sisterID := createUser(t, env, auth.RoleStudent, "sister", "Козлова Мария Ильинична")
	require.NoError(t, env.School.AddChild(ctx, parentID, sisterID))

	body = servertest.Get(t, env.Handler, "/", parent).Body.String()
	assert.Contains(t, body, `aria-label="Дети"`)
	assert.Contains(t, body, `href="/?child=`+strconv.FormatInt(sisterID, 10)+`"`)
	assert.Contains(t, body, `href="/?child=`+strconv.FormatInt(studentID, 10)+`" aria-current="page"`)
	assert.Contains(t, body, "Средний: 5,00")

	body = servertest.Get(t, env.Handler, "/?child="+strconv.FormatInt(sisterID, 10), parent).Body.String()
	assert.Contains(t, body, `href="/?child=`+strconv.FormatInt(sisterID, 10)+`" aria-current="page"`)
	assert.Contains(t, body, "За последние семь дней оценок нет")

	stranger := createUser(t, env, auth.RoleStudent, "stranger", "Чужой Ученик Иванович")
	foreign := servertest.Get(t, env.Handler, "/?child="+strconv.FormatInt(stranger, 10), parent)
	assert.Equal(t, http.StatusNotFound, foreign.Code)

	body = servertest.Get(t, env.Handler, "/", env.LoginAs(t, "childless")).Body.String()
	assert.Contains(t, body, "К вашему аккаунту не привязаны дети")
}

func createUser(t *testing.T, env *servertest.Env, role auth.Role, login, fullName string) int64 {
	t.Helper()

	env.CreateUser(t, role, login, fullName)

	users, err := env.Auth.Users(t.Context(), auth.UserFilter{Role: role, IncludeInactive: true})
	require.NoError(t, err)

	for _, user := range users {
		if user.Login == login {
			return user.ID
		}
	}

	t.Fatalf("user %q not found", login)

	return 0
}
