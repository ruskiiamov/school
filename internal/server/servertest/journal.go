package servertest

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/validation"
)

func (env *Env) CreateLesson(t *testing.T, teacherID, classID, subjectID int64, date time.Time) int64 {
	t.Helper()

	class, err := env.School.ClassByID(t.Context(), classID)
	require.NoError(t, err)

	start, end := env.School.YearBounds(class.Year)

	id, err := env.Journal.CreateLesson(t.Context(), teacherID, classID, subjectID, date.Format(validation.DateLayout), journal.YearBounds{Start: start, End: end})
	require.NoError(t, err)

	return id
}

func (env *Env) SetMark(t *testing.T, teacherID, lessonID, studentID, workTypeID int64, value string) {
	t.Helper()

	env.saveEntry(t, teacherID, lessonID, studentID, func(input *journal.MarksEntryInput) {
		input.Values[workTypeID] = value
	})
}

func (env *Env) SetCommentedMark(t *testing.T, teacherID, lessonID, studentID, workTypeID int64, value, comment string) {
	t.Helper()

	env.saveEntry(t, teacherID, lessonID, studentID, func(input *journal.MarksEntryInput) {
		input.Values[workTypeID] = value
		input.Comments[workTypeID] = comment
	})
}

func (env *Env) SetRecord(t *testing.T, teacherID, lessonID, studentID int64, absent bool, comment string) {
	t.Helper()

	env.saveEntry(t, teacherID, lessonID, studentID, func(input *journal.MarksEntryInput) {
		input.Absent = absent
		input.Comment = comment
	})
}

func (env *Env) saveEntry(t *testing.T, teacherID, lessonID, studentID int64, change func(*journal.MarksEntryInput)) {
	t.Helper()

	entry, err := env.Journal.MarksEntry(t.Context(), teacherID, lessonID, studentID)
	require.NoError(t, err)

	input := journal.MarksEntryInput{Values: map[int64]string{}, Comments: map[int64]string{}, Absent: entry.Absent, Comment: entry.Comment}

	for _, workType := range entry.Types {
		input.Values[workType.ID] = workType.Value
		input.Comments[workType.ID] = workType.Comment
	}

	known := len(input.Values)

	change(&input)

	require.Len(t, input.Values, known, "work type is not offered for this lesson")

	require.NoError(t, env.Journal.SaveMarksEntry(t.Context(), teacherID, lessonID, studentID, input))
}

var WorkTypeNames = []string{
	"Ответ на уроке",
	"Классная работа",
	"Самостоятельная работа",
	"Контрольная работа",
	"Домашняя работа (письменная)",
	"Домашняя работа (устная)",
}

func (env *Env) EnsureWorkTypes(t *testing.T, teacherID, subjectID int64) []journal.WorkType {
	t.Helper()

	existing, err := env.Journal.WorkTypes(t.Context(), teacherID, subjectID, true)
	require.NoError(t, err)

	if len(existing) == 0 {
		for _, name := range WorkTypeNames {
			_, err := env.Journal.CreateWorkType(t.Context(), teacherID, subjectID, name, false)
			require.NoError(t, err)
		}
	}

	workTypes, err := env.Journal.WorkTypes(t.Context(), teacherID, subjectID, false)
	require.NoError(t, err)

	return workTypes
}

func (env *Env) EnablePenalties(t *testing.T) {
	t.Helper()

	require.NoError(t, env.School.SavePenaltySettings(t.Context(), school.PenaltySettingsInput{
		Enabled: true,
		Name:    "Мытьё полов",
		Unit:    "м²",
		Full:    "20",
	}))
}

func (env *Env) DisablePenalties(t *testing.T) {
	t.Helper()

	settings := env.School.PenaltySettings()

	require.NoError(t, env.School.SavePenaltySettings(t.Context(), school.PenaltySettingsInput{
		Name: settings.Name,
		Unit: settings.Unit,
		Full: strconv.Itoa(settings.Full),
	}))
}

func (env *Env) SetPenalty(t *testing.T, teacherID, lessonID, studentID, workTypeID int64, done string) {
	t.Helper()

	env.saveEntry(t, teacherID, lessonID, studentID, func(input *journal.MarksEntryInput) {
		input.Done = map[int64]string{workTypeID: done}
		input.PenaltyFull = env.School.PenaltySettings().Full
	})
}

func (env *Env) AddWorkoff(t *testing.T, recordedBy, studentID int64, points string) {
	t.Helper()

	_, err := env.Journal.AddWorkoff(t.Context(), recordedBy, studentID, journal.WorkoffInput{
		Date:   env.School.Today().Format(validation.DateLayout),
		Points: points,
	}, env.School.Today())
	require.NoError(t, err)
}
