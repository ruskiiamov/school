package admin_test

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/backup"
	"github.com/ruskiiamov/school/internal/server/servertest"
)

func TestBackupPage(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	_, err := env.Files.Save(strings.NewReader("Упр. 12"), 1<<20)
	require.NoError(t, err)

	recorder := servertest.Get(t, env.Handler, "/admin/backup", servertest.Login(t, env.Handler))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "Резервная копия")
	assert.Contains(t, body, "1 файл, 10 Б")
	assert.Contains(t, body, `href="/admin/backup/download"`)
	assert.Contains(t, body, "Как восстановить")
}

func TestBackupDownload(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	saved, err := env.Files.Save(strings.NewReader("Упр. 12"), 1<<20)
	require.NoError(t, err)

	recorder := servertest.Get(t, env.Handler, "/admin/backup/download", servertest.Login(t, env.Handler))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/gzip", recorder.Header().Get("Content-Type"))
	assert.Regexp(t, `^attachment; filename=school-backup-\d{4}-\d{2}-\d{2}\.tar\.gz$`, recorder.Header().Get("Content-Disposition"))

	gz, err := gzip.NewReader(recorder.Body)
	require.NoError(t, err)

	entries := map[string]int64{}
	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		entries[header.Name] = header.Size
	}

	require.Len(t, entries, 3)
	assert.Contains(t, entries, backup.FilesDir+"/")
	assert.Positive(t, entries[backup.DatabaseName])
	assert.Equal(t, saved.Size, entries[backup.FilesDir+"/"+saved.ID])
}

func TestBackupForbiddenToOtherRoles(t *testing.T) {
	t.Parallel()

	env := servertest.New(t)
	env.CreateUser(t, auth.RoleTeacher, "teacher", "Сидорова Анна Андреевна")
	cookie := env.LoginAs(t, "teacher")

	for _, path := range []string{"/admin/backup", "/admin/backup/download"} {
		recorder := servertest.Get(t, env.Handler, path, cookie)
		assert.Equal(t, http.StatusNotFound, recorder.Code, path)
	}

	recorder := servertest.Get(t, env.Handler, "/admin/backup")
	servertest.AssertRedirect(t, recorder, "/login")
}
