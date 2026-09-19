package journal_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/servertest"
	"github.com/ruskiiamov/school/internal/validation"
)

func TestLessonSurvivesEndedSubstitution(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))

	substitute := createUser(t, f.env, auth.RoleTeacher, "substitute", "Кузнецов Виктор Павлович")
	student := createUser(t, f.env, auth.RoleStudent, "student", "Козлов Пётр Ильич")
	require.NoError(t, f.env.School.AddClassStudent(ctx, f.class, student))

	substitutionID, err := f.env.School.CreateSubstitution(ctx, school.SubstitutionInput{
		ClassID: f.class, SubjectID: f.subject, TeacherID: substitute, StartDate: dateValue(f.today),
	})
	require.NoError(t, err)

	substituteCookie := f.env.LoginAs(t, "substitute")
	lesson := openLesson(t, f.env, substituteCookie, f.class, f.subject, f.today)

	workTypes, err := f.env.School.WorkTypes(ctx, false)
	require.NoError(t, err)
	_, err = f.env.Journal.AddMark(ctx, substitute, lesson, student, journalMarkInput(workTypes[0].ID, 5))
	require.NoError(t, err)

	var refused validation.Errors
	require.ErrorAs(t, f.env.School.DeleteSubstitution(ctx, substitutionID), &refused)
	assert.Contains(t, refused, "end_date")

	period := school.Period{StartDate: dateValue(f.today), EndDate: dateValue(f.today)}
	require.NoError(t, f.env.School.UpdateSubstitutionPeriod(ctx, substitutionID, period))

	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, lessonPath(lesson, ""), substituteCookie).Code)
	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, lessonPath(lesson, ""), f.env.LoginAs(t, "teacher")).Code)

	_, err = f.env.Journal.AddMark(ctx, substitute, lesson, student, journalMarkInput(workTypes[0].ID, 4))
	require.NoError(t, err)

	body := servertest.Get(t, f.env.Handler, "/journal", substituteCookie).Body.String()
	assert.Contains(t, body, `href="`+lessonPath(lesson, "")+`"`)

	again := servertest.PostForm(t, f.env.Handler, "/journal",
		url.Values{"pair": {pairValue(f.class, f.subject)}, "date": {dateValue(f.today.AddDate(0, 0, 1))}},
		[]*http.Cookie{substituteCookie}, nil)
	assert.Equal(t, http.StatusOK, again.Code)
	assert.NotContains(t, again.Header().Get("Location"), "/journal/lessons/")

	diary := servertest.Get(t, f.env.Handler, "/diary", f.env.LoginAs(t, "student")).Body.String()
	assert.Contains(t, diary, "Кузнецов В.")
}
