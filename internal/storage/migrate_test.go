package storage

import (
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/storage/migrations"
)

func TestUserNamesMigrationSplitsFullName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := Open(ctx, config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	require.NoError(t, err)

	_, err = provider.UpTo(ctx, 4)
	require.NoError(t, err)

	for login, fullName := range map[string]string{
		"three":  "Сидорова Анна Андреевна",
		"two":    "Петров Иван",
		"one":    "Администратор",
		"spaces": "  Козлова   Мария  Ильинична оглы ",
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (login, password_hash, full_name, role, active, created_at, updated_at)
			VALUES (?, 'hash', ?, 'teacher', 1, 0, 0)`, login, fullName)
		require.NoError(t, err)
	}

	_, err = provider.Up(ctx)
	require.NoError(t, err)

	repo := NewUserRepo(db)

	tests := []struct {
		login  string
		last   string
		first  string
		middle string
	}{
		{"three", "Сидорова", "Анна", "Андреевна"},
		{"two", "Петров", "Иван", ""},
		{"one", "Администратор", "", ""},
		{"spaces", "Козлова", "Мария", "Ильинична оглы"},
	}

	for _, tt := range tests {
		user, err := repo.ByLogin(ctx, tt.login)
		require.NoError(t, err, tt.login)
		assert.Equal(t, []string{tt.last, tt.first, tt.middle}, []string{user.LastName, user.FirstName, user.MiddleName}, tt.login)
	}
}

func TestMarksMigrationAllowsOneMarkPerType(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := Open(ctx, config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	require.NoError(t, err)

	_, err = provider.Up(ctx)
	require.NoError(t, err)

	insert := func(mark Mark) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (?, ?, ?, ?, 0, 0)`,
			mark.LessonID, mark.StudentID, mark.WorkTypeID, mark.Value)

		return err
	}

	require.NoError(t, insert(Mark{LessonID: 1, StudentID: 1, WorkTypeID: 1, Value: "5"}))
	require.Error(t, insert(Mark{LessonID: 1, StudentID: 1, WorkTypeID: 1, Value: "4"}))

	for _, mark := range []Mark{
		{LessonID: 1, StudentID: 1, WorkTypeID: 2, Value: "4"},
		{LessonID: 1, StudentID: 2, WorkTypeID: 1, Value: "3"},
		{LessonID: 2, StudentID: 1, WorkTypeID: 1, Value: "2"},
	} {
		require.NoError(t, insert(mark))
	}

	repo := NewMarkRepo(db)

	marks, err := repo.ListByLesson(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, marks, 3)
}

func TestMarkValuesMigrationKeepsDigitsAsText(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := Open(ctx, config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	require.NoError(t, err)

	_, err = provider.UpTo(ctx, 7)
	require.NoError(t, err)

	for _, value := range []int{5, 3} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (1, ?, 1, ?, 0, 0)`,
			value, value)
		require.NoError(t, err)
	}

	_, err = provider.UpTo(ctx, 8)
	require.NoError(t, err)

	marks, err := NewMarkRepo(db).ListByLesson(ctx, 1)
	require.NoError(t, err)
	require.Len(t, marks, 2)
	assert.Equal(t, []string{"5", "3"}, []string{marks[0].Value, marks[1].Value})
	assert.Empty(t, marks[0].Comment)

	var kind string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT typeof(value) FROM marks LIMIT 1").Scan(&kind))
	assert.Equal(t, "text", kind)

	_, err = db.ExecContext(ctx,
		`INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (1, 5, 1, '4', 0, 0)`)
	require.Error(t, err)
}

func TestTeacherWorkTypesMigrationCopiesSharedTypes(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := Open(ctx, config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	require.NoError(t, err)

	_, err = provider.UpTo(ctx, 8)
	require.NoError(t, err)

	exec := func(query string, args ...any) {
		t.Helper()

		_, err := db.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}

	exec("UPDATE work_types SET active = 0 WHERE name = 'Классная работа'")
	exec("INSERT INTO teaching_assignments (teacher_id, subject_id, class_id) VALUES (10, 100, 1)")
	exec("INSERT INTO substitutions (class_id, subject_id, teacher_id, start_date, created_at, updated_at) VALUES (1, 100, 20, '2026-10-01', 0, 0)")
	exec("INSERT INTO lessons (id, class_id, subject_id, teacher_id, date, created_at, updated_at) VALUES (1, 1, 100, 10, '2026-09-15', 0, 0)")
	exec("INSERT INTO lessons (id, class_id, subject_id, teacher_id, date, created_at, updated_at) VALUES (2, 1, 100, 20, '2026-10-02', 0, 0)")
	exec("INSERT INTO lessons (id, class_id, subject_id, teacher_id, date, created_at, updated_at) VALUES (3, 2, 200, 10, '2026-09-16', 0, 0)")

	var answer, control int64
	require.NoError(t, db.QueryRowContext(ctx, "SELECT id FROM work_types WHERE name = 'Ответ на уроке'").Scan(&answer))
	require.NoError(t, db.QueryRowContext(ctx, "SELECT id FROM work_types WHERE name = 'Контрольная работа'").Scan(&control))

	exec("INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (1, 1, ?, '5', 0, 0)", answer)
	exec("INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (2, 1, ?, '4', 0, 0)", control)
	exec("INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (3, 1, ?, '3', 0, 0)", answer)
	exec("INSERT INTO marks (lesson_id, student_id, work_type_id, value, created_at, updated_at) VALUES (99, 1, ?, '2', 0, 0)", answer)

	_, err = provider.Up(ctx)
	require.NoError(t, err)

	repo := NewWorkTypeRepo(db)

	for _, owner := range []struct{ teacher, subject int64 }{{10, 100}, {20, 100}, {10, 200}} {
		all, err := repo.ListByOwner(ctx, owner.teacher, owner.subject, true)
		require.NoError(t, err)
		require.Len(t, all, 6, owner)

		active, err := repo.ListByOwner(ctx, owner.teacher, owner.subject, false)
		require.NoError(t, err)
		assert.Len(t, active, 5, owner)
		assert.Equal(t, "Ответ на уроке", active[0].Name)
	}

	var shared int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM work_types WHERE teacher_id = 0").Scan(&shared))
	assert.Zero(t, shared)

	marks := NewMarkRepo(db)

	for _, tt := range []struct {
		lesson  int64
		teacher int64
		subject int64
		name    string
	}{
		{1, 10, 100, "Ответ на уроке"},
		{2, 20, 100, "Контрольная работа"},
		{3, 10, 200, "Ответ на уроке"},
	} {
		list, err := marks.ListByLesson(ctx, tt.lesson)
		require.NoError(t, err)
		require.Len(t, list, 1)

		workType, err := repo.ByID(ctx, list[0].WorkTypeID)
		require.NoError(t, err)
		assert.Equal(t, []any{tt.teacher, tt.subject, tt.name}, []any{workType.TeacherID, workType.SubjectID, workType.Name})
	}

	orphans, err := marks.ListByLesson(ctx, 99)
	require.NoError(t, err)
	assert.Empty(t, orphans)
}

func TestDefaultWorkTypesCreatedOnce(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := Open(ctx, config.DB{Path: filepath.Join(t.TempDir(), "test.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, Migrate(ctx, db))

	repo := NewWorkTypeRepo(db)

	created, err := repo.CreateDefaults(ctx, 1, 2, []string{"А", "Б"})
	require.NoError(t, err)
	assert.True(t, created)

	list, err := repo.ListByOwner(ctx, 1, 2, true)
	require.NoError(t, err)
	require.Len(t, list, 2)

	require.NoError(t, repo.SetActive(ctx, list[0].ID, false))
	require.NoError(t, repo.SetActive(ctx, list[1].ID, false))

	created, err = repo.CreateDefaults(ctx, 1, 2, []string{"А", "Б"})
	require.NoError(t, err)
	assert.False(t, created)

	active, err := repo.ListByOwner(ctx, 1, 2, false)
	require.NoError(t, err)
	assert.Empty(t, active)
}
