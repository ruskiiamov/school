package journal_test

import (
	"html"
	"net/http"
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
	assert.NotContains(t, body, "Всего:")

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

	_, err = f.env.Journal.AddMark(ctx, f.teacher, today, maria, journalMarkInput(test.ID, 4))
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, today, maria, journalMarkInput(answer.ID, 5))
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, earlier, maria, journalMarkInput(answer.ID, 4))
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, f.teacher, earlier, gone, journalMarkInput(answer.ID, 3))
	require.NoError(t, err)
	require.NoError(t, f.env.Journal.SaveRecord(ctx, f.teacher, today, petr, journal.RecordInput{Absent: true, Comment: "Болел"}))
	require.NoError(t, f.env.School.RemoveClassStudent(ctx, f.class, gone))
	require.NoError(t, f.env.Auth.SetUserActive(ctx, petr, false))

	body = servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), teacher).Body.String()
	assert.NotContains(t, body, "Уроков пока нет")
	assert.Contains(t, body, "Всего: 2 урока")
	assert.Contains(t, body, "Как работать с журналом")
	assert.Contains(t, body, `id="lesson-`+strconv.FormatInt(today, 10)+`"`)
	assert.Less(t, strings.Index(body, `id="lesson-`+strconv.FormatInt(today, 10)+`"`), strings.Index(body, `id="lesson-`+strconv.FormatInt(earlier, 10)+`"`))
	assert.Contains(t, body, view.FormatShortDate(f.today))
	assert.Contains(t, body, "Квадратные уравнения")

	assert.Less(t, strings.Index(body, "<span>Иванова</span> <span>Мария</span>"), strings.Index(body, "<span>Козлов</span> <span>Пётр</span>"))
	assert.Less(t, strings.Index(body, "<span>Козлов</span>"), strings.Index(body, "<span>Ушедший</span>"))
	assert.Contains(t, body, `title="Козлов Пётр Ильич (удалён)"`)
	assert.Contains(t, body, `title="Ушедший Олег Олегович (не в классе)"`)
	assert.Contains(t, body, ">4,33<")

	mariaToday := html.EscapeString(lessonPath(today, "?student="+strconv.FormatInt(maria, 10)))
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
	assert.Contains(t, body, "замена · Кузнецов В. П.")
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
