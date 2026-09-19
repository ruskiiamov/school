package journal_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestJournalListsAllLessonsByPages(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	require.NoError(t, f.env.School.AssignTeacher(ctx, f.class, f.subject, f.teacher))
	teacher := f.env.LoginAs(t, "teacher")

	start, _ := f.env.School.YearBounds(f.env.School.CurrentYear())
	ids := make([]int64, 0, 23)

	for day := range 23 {
		id, err := f.env.Journal.OpenLesson(ctx, f.teacher, f.env.School.CurrentYear(), start.AddDate(0, 0, 30),
			journal.OpenLessonInput{ClassID: f.class, SubjectID: f.subject, Date: dateValue(start.AddDate(0, 0, day))})
		require.NoError(t, err)

		ids = append(ids, id)
	}

	first := servertest.Get(t, f.env.Handler, "/journal", teacher).Body.String()
	assert.Contains(t, first, "Страница 1 из 3")
	assert.Contains(t, first, `href="`+lessonPath(ids[22], "")+`"`)
	assert.Contains(t, first, `href="`+lessonPath(ids[13], "")+`"`)
	assert.NotContains(t, first, `href="`+lessonPath(ids[12], "")+`"`)
	assert.Contains(t, first, `href="/journal?page=2"`)
	assert.NotContains(t, first, "← Новее")

	second := servertest.Get(t, f.env.Handler, "/journal?page=2", teacher).Body.String()
	assert.Contains(t, second, "Страница 2 из 3")
	assert.Contains(t, second, `href="`+lessonPath(ids[12], "")+`"`)
	assert.Contains(t, second, `href="`+lessonPath(ids[3], "")+`"`)
	assert.Contains(t, second, `href="/journal"`)
	assert.Contains(t, second, `href="/journal?page=3"`)

	third := servertest.Get(t, f.env.Handler, "/journal?page=3", teacher).Body.String()
	assert.Contains(t, third, "Страница 3 из 3")
	assert.Contains(t, third, `href="`+lessonPath(ids[0], "")+`"`)
	assert.NotContains(t, third, "Старее →")

	servertest.AssertRedirect(t, servertest.Get(t, f.env.Handler, "/journal?page=9", teacher), "/journal?page=3")
	assert.Equal(t, http.StatusOK, servertest.Get(t, f.env.Handler, "/journal?page=abc", teacher).Code)
}
