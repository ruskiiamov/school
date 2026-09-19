package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/backup"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/files"
	"github.com/ruskiiamov/school/internal/storage"
)

func TestArchiveContainsDatabaseCopyAndFiles(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "school.db")

	db, err := storage.Open(ctx, config.DB{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, storage.Migrate(ctx, db))

	_, err = db.ExecContext(ctx, "INSERT INTO subjects (name, active, created_at, updated_at) VALUES ('Алгебра', 1, 0, 0)")
	require.NoError(t, err)

	store, err := files.NewStore(filepath.Join(dataDir, "files"))
	require.NoError(t, err)
	saved, err := store.Save(strings.NewReader("Упр. 12"), 1<<20)
	require.NoError(t, err)

	service := backup.New(dbPath, store)

	usage, err := service.Usage()
	require.NoError(t, err)
	assert.Equal(t, backup.Usage{FileCount: 1, FileBytes: saved.Size}, usage)

	archive, err := service.Prepare(ctx)
	require.NoError(t, err)

	var buf bytes.Buffer
	written, err := archive.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	assert.Equal(t, int64(buf.Len()), written)

	entries := untar(t, &buf)
	require.Len(t, entries, 2)
	assert.Equal(t, "Упр. 12", string(entries[backup.FilesDir+"/"+saved.ID]))

	copyPath := filepath.Join(t.TempDir(), "copy.db")
	require.NoError(t, os.WriteFile(copyPath, entries[backup.DatabaseName], 0o600))

	restored, err := storage.Open(ctx, config.DB{Path: copyPath})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })

	var name string
	require.NoError(t, restored.QueryRowContext(ctx, "SELECT name FROM subjects").Scan(&name))
	assert.Equal(t, "Алгебра", name)

	leftovers, err := filepath.Glob(filepath.Join(dataDir, "backup-*.db"))
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

func TestPrepareFailsWithoutDatabase(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	store, err := files.NewStore(filepath.Join(dataDir, "files"))
	require.NoError(t, err)

	_, err = backup.New(filepath.Join(dataDir, "missing.db"), store).Prepare(t.Context())
	require.Error(t, err)

	leftovers, err := filepath.Glob(filepath.Join(dataDir, "backup-*.db"))
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

func untar(t *testing.T, r io.Reader) map[string][]byte {
	t.Helper()

	gz, err := gzip.NewReader(r)
	require.NoError(t, err)

	entries := map[string][]byte{}
	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		require.NoError(t, err)

		body, err := io.ReadAll(tr)
		require.NoError(t, err)

		entries[header.Name] = body
	}
}
