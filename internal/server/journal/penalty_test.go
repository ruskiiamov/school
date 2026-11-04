package journal_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

type penaltyFixture struct {
	marksFixture
	homework int64
}

func newPenaltyFixture(t *testing.T) penaltyFixture {
	t.Helper()

	f := newMarksFixture(t)
	f.env.EnablePenalties(t)

	homework, err := f.env.Journal.CreateWorkType(t.Context(), f.teacher, f.subject, "Домашняя работа", true)
	require.NoError(t, err)

	return penaltyFixture{marksFixture: f, homework: homework}
}

func doneField(workTypeID, studentID int64) string {
	return journal.MarkKey{WorkTypeID: workTypeID, StudentID: studentID}.DoneField()
}

func entryPath(lessonID, studentID int64) string {
	return lessonPath(lessonID, "/students/"+strconv.FormatInt(studentID, 10)+"/marks")
}

func TestWorkTypePenalizedFlag(t *testing.T) {
	t.Parallel()

	f := newMarksFixture(t)
	base := "/journal/subjects/" + strconv.FormatInt(f.subject, 10) + "/work-types"

	recorder := servertest.Get(t, f.env.Handler, base, f.cookie)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "Со штрафом")

	recorder = servertest.PostForm(t, f.env.Handler, base, url.Values{"name": {"Проект"}, "penalized": {"1"}}, []*http.Cookie{f.cookie}, nil)
	servertest.AssertRedirect(t, recorder, base)

	f.env.EnablePenalties(t)

	recorder = servertest.PostForm(t, f.env.Handler, base, url.Values{"name": {"Домашняя работа"}, "penalized": {"1"}}, []*http.Cookie{f.cookie}, nil)
	servertest.AssertRedirect(t, recorder, base)

	workTypes, err := f.env.Journal.WorkTypes(t.Context(), f.teacher, f.subject, false)
	require.NoError(t, err)

	flags := map[string]bool{}
	for _, workType := range workTypes {
		flags[workType.Name] = workType.Penalized
	}

	assert.False(t, flags["Проект"])
	assert.True(t, flags["Домашняя работа"])

	body := servertest.Get(t, f.env.Handler, base, f.cookie).Body.String()
	assert.Contains(t, body, "Со штрафом")
	assert.Contains(t, body, "со штрафом")
}

func TestWorkTypeRenameKeepsFlagWhilePenaltiesDisabled(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	base := "/journal/subjects/" + strconv.FormatInt(f.subject, 10) + "/work-types"

	f.env.DisablePenalties(t)

	recorder := servertest.PostForm(t, f.env.Handler, base+"/"+strconv.FormatInt(f.homework, 10),
		url.Values{"name": {"Домашка"}}, []*http.Cookie{f.cookie}, nil)
	servertest.AssertRedirect(t, recorder, base)

	workType, err := f.env.Journal.OwnWorkType(t.Context(), f.teacher, f.homework)
	require.NoError(t, err)
	assert.Equal(t, "Домашка", workType.Name)
	assert.True(t, workType.Penalized)
}

func TestMarksEntrySavesPenalty(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	path := entryPath(f.lesson, f.maria)
	field := journal.EntryDoneKey(f.homework)

	body := servertest.Get(t, f.env.Handler, path, f.cookie).Body.String()
	assert.Contains(t, body, `name="`+field+`"`)
	assert.Equal(t, 1, strings.Count(body, "выполнено, %"))

	recorder := servertest.PostForm(t, f.env.Handler, path, url.Values{field: {"30"}}, []*http.Cookie{f.cookie}, nil)
	require.Equal(t, http.StatusSeeOther, recorder.Code, recorder.Body.String())

	body = servertest.Get(t, f.env.Handler, path, f.cookie).Body.String()
	assert.Contains(t, body, `value="30"`)
	assert.Contains(t, body, "14 м²")
	assert.Contains(t, body, "Мытьё полов — долг: 14 м²")

	journalBody := servertest.Get(t, f.env.Handler, pairPath(f.class, f.subject), f.cookie).Body.String()
	assert.Contains(t, journalBody, "Домашняя работа: выполнено 30 %, 14 м²")
	assert.Contains(t, journalBody, `<span class="text-amber-600">%</span>`)

	recorder = servertest.PostForm(t, f.env.Handler, path, url.Values{field: {"100"}}, []*http.Cookie{f.cookie}, nil)
	require.Equal(t, http.StatusSeeOther, recorder.Code)

	debt, err := f.env.Journal.StudentDebt(t.Context(), f.maria)
	require.NoError(t, err)
	assert.Zero(t, debt)
}

func TestMarksEntryRejectsInvalidPercent(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	path := entryPath(f.lesson, f.maria)
	field := journal.EntryDoneKey(f.homework)

	for _, value := range []string{"101", "-1", "30,5", "abc"} {
		recorder := servertest.PostForm(t, f.env.Handler, path,
			url.Values{field: {value}, journal.EntryTypeKey(f.answer.ID): {"5"}}, []*http.Cookie{f.cookie}, nil)
		require.Equal(t, http.StatusOK, recorder.Code, value)
		assert.Contains(t, recorder.Body.String(), "Выполнено: целое число от 0 до 100", value)
	}

	marks := lessonMarks(t, f.fixture, f.lesson)
	assert.Empty(t, marks.Marks)
	assert.Empty(t, marks.Penalties)
}

func TestMarksEntryIgnoresPenaltyWhenDisabled(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	f.env.DisablePenalties(t)

	path := entryPath(f.lesson, f.maria)
	field := journal.EntryDoneKey(f.homework)

	assert.NotContains(t, servertest.Get(t, f.env.Handler, path, f.cookie).Body.String(), field)

	recorder := servertest.PostForm(t, f.env.Handler, path, url.Values{field: {"0"}}, []*http.Cookie{f.cookie}, nil)
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Empty(t, lessonMarks(t, f.fixture, f.lesson).Penalties)
}

func TestLessonMarksSavesPenaltyCell(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	action := lessonPath(f.lesson, "/marks")

	body := servertest.Get(t, f.env.Handler, action, f.cookie, cellHeaders()).Body.String()
	assert.Contains(t, body, `name="`+doneField(f.homework, f.maria)+`"`)
	assert.NotContains(t, body, doneField(f.answer.ID, f.maria))

	recorder := servertest.PostForm(t, f.env.Handler, action, url.Values{doneField(f.homework, f.maria): {"33"}},
		[]*http.Cookie{f.cookie}, cellHeaders())
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `value="33"`)
	assert.Contains(t, recorder.Body.String(), "hx-swap-oob")

	penalty := lessonMarks(t, f.fixture, f.lesson).Penalties[journal.MarkKey{WorkTypeID: f.homework, StudentID: f.maria}]
	assert.Equal(t, journal.Penalty{DonePercent: 33, Points: 13, Status: journal.PenaltyOpen}, penalty)

	recorder = servertest.PostForm(t, f.env.Handler, action, url.Values{doneField(f.answer.ID, f.maria): {"50"}},
		[]*http.Cookie{f.cookie}, cellHeaders())
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestPaidPenaltyIsLocked(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "0")
	f.env.AddWorkoff(t, f.teacher, f.maria, "5")

	path := entryPath(f.lesson, f.maria)
	body := servertest.Get(t, f.env.Handler, path, f.cookie).Body.String()
	assert.Contains(t, body, "0 % · 20 м² · частично")
	assert.NotContains(t, body, `name="`+journal.EntryDoneKey(f.homework)+`"`)

	recorder := servertest.PostForm(t, f.env.Handler, path, url.Values{journal.EntryDoneKey(f.homework): {"50"}}, []*http.Cookie{f.cookie}, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Штраф уже погашен, изменить нельзя")

	recorder = servertest.PostForm(t, f.env.Handler, lessonPath(f.lesson, "/marks"), url.Values{doneField(f.homework, f.maria): {"50"}},
		[]*http.Cookie{f.cookie}, cellHeaders())
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Штраф уже погашен, изменить нельзя")

	debt, err := f.env.Journal.StudentDebt(t.Context(), f.maria)
	require.NoError(t, err)
	assert.Equal(t, 15, debt)
}

func TestLessonWithPenaltyIsNotDeleted(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "50")

	recorder := servertest.PostForm(t, f.env.Handler, lessonPath(f.lesson, "/date"), url.Values{"date": {""}}, []*http.Cookie{f.cookie}, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Урок с оценками или записями удалить нельзя")
	assert.Len(t, pairLessons(t, f.fixture), 1)
}

func TestAdminJournalsShowPenalties(t *testing.T) {
	t.Parallel()

	f := newPenaltyFixture(t)
	f.env.SetPenalty(t, f.teacher, f.lesson, f.maria, f.homework, "30")
	admin := servertest.Login(t, f.env.Handler)

	entry := servertest.Get(t, f.env.Handler, "/admin/journals/lessons/"+strconv.FormatInt(f.lesson, 10)+"/students/"+strconv.FormatInt(f.maria, 10), admin)
	require.Equal(t, http.StatusOK, entry.Code)
	assert.Contains(t, entry.Body.String(), "Выполнено 30 % · 14 м² · не погашено")

	marks := servertest.Get(t, f.env.Handler, "/admin/journals/lessons/"+strconv.FormatInt(f.lesson, 10)+"/marks", admin, cellHeaders())
	require.Equal(t, http.StatusOK, marks.Code)
	assert.Contains(t, marks.Body.String(), "30 %")
	assert.NotContains(t, marks.Body.String(), `name="`+doneField(f.homework, f.maria)+`"`)
}
