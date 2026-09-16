package files_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/files"
)

func TestStoreSaveOpenDelete(t *testing.T) {
	t.Parallel()

	store, err := files.NewStore(t.TempDir())
	require.NoError(t, err)

	saved, err := store.Save(strings.NewReader("<html><body>hi</body></html>"), 100)
	require.NoError(t, err)
	assert.Len(t, saved.ID, 32)
	assert.Equal(t, int64(28), saved.Size)
	assert.Equal(t, "text/html; charset=utf-8", saved.ContentType)

	file, err := store.Open(saved.ID)
	require.NoError(t, err)

	content, err := io.ReadAll(file)
	require.NoError(t, file.Close())
	require.NoError(t, err)
	assert.Equal(t, "<html><body>hi</body></html>", string(content))

	ids, err := store.IDs()
	require.NoError(t, err)
	assert.Equal(t, []string{saved.ID}, ids)

	require.NoError(t, store.Delete(saved.ID))
	assert.ErrorIs(t, store.Delete(saved.ID), files.ErrNotFound)

	_, err = store.Open(saved.ID)
	assert.ErrorIs(t, err, files.ErrNotFound)
}

func TestStoreSaveTooLarge(t *testing.T) {
	t.Parallel()

	store, err := files.NewStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.Save(strings.NewReader(strings.Repeat("a", 1000)), 999)
	assert.ErrorIs(t, err, files.ErrTooLarge)

	saved, err := store.Save(strings.NewReader(strings.Repeat("a", 999)), 999)
	require.NoError(t, err)
	assert.Equal(t, int64(999), saved.Size)

	ids, err := store.IDs()
	require.NoError(t, err)
	assert.Equal(t, []string{saved.ID}, ids)
}

func TestStoreRejectsForeignIDs(t *testing.T) {
	t.Parallel()

	store, err := files.NewStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.Open("../store.go")
	assert.ErrorIs(t, err, files.ErrNotFound)
	assert.ErrorIs(t, store.Delete(""), files.ErrNotFound)
}
