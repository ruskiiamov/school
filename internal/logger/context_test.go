package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHandlerAddsRequestID(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	log := slog.New(NewContextHandler(slog.NewJSONHandler(buf, nil)))

	log.InfoContext(WithRequestID(t.Context(), "abc123"), "with id")
	log.InfoContext(t.Context(), "without id")

	var entries []map[string]any

	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		require.NoError(t, json.Unmarshal(line, &entry))

		entries = append(entries, entry)
	}

	require.Len(t, entries, 2)
	assert.Equal(t, "abc123", entries[0]["request_id"])
	assert.NotContains(t, entries[1], "request_id")
}

func TestRequestIDMissing(t *testing.T) {
	t.Parallel()

	assert.Empty(t, RequestID(t.Context()))
}
