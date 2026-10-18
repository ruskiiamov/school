package servertest

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/journal"
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

func (env *Env) SetMark(t *testing.T, teacherID, lessonID, studentID, workTypeID int64, value int) {
	t.Helper()

	env.saveEntry(t, teacherID, lessonID, studentID, func(input *journal.MarksEntryInput) {
		input.Values[workTypeID] = strconv.Itoa(value)
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

	input := journal.MarksEntryInput{Values: map[int64]string{}, Absent: entry.Absent, Comment: entry.Comment}

	for _, workType := range entry.Types {
		if workType.Value != 0 {
			input.Values[workType.ID] = strconv.Itoa(workType.Value)
		}
	}

	change(&input)

	require.NoError(t, env.Journal.SaveMarksEntry(t.Context(), teacherID, lessonID, studentID, input))
}
