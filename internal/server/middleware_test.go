package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/logger"
)

func TestPanicInPageIsLoggedAsRequest(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	log := slog.New(logger.NewContextHandler(slog.NewJSONHandler(buf, nil)))
	s := New(&config.Config{}, nil, nil, nil, log)

	handler := chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}), requestID, s.logRequests, s.recoverPanic)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	var entries []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))

		entries = append(entries, entry)
	}

	require.Len(t, entries, 2)
	assert.Equal(t, "panic in handler", entries[0]["msg"])
	assert.Equal(t, "request", entries[1]["msg"])
	assert.InDelta(t, http.StatusInternalServerError, entries[1]["status"], 0)
	assert.Equal(t, entries[0]["request_id"], entries[1]["request_id"])
	assert.NotEmpty(t, entries[1]["request_id"])
}

func TestInsecureCookieWarnedOnceBehindHTTPSProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		secure   bool
		proto    string
		wantWarn bool
	}{
		{name: "https proxy and insecure cookie", secure: false, proto: "https", wantWarn: true},
		{name: "https proxy and secure cookie", secure: true, proto: "https", wantWarn: false},
		{name: "plain http", secure: false, proto: "", wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			log := slog.New(slog.NewJSONHandler(buf, nil))
			s := New(&config.Config{Session: config.Session{Secure: tt.secure}}, nil, nil, nil, log)

			handler := s.warnInsecureCookie(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			for range 2 {
				req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
				if tt.proto != "" {
					req.Header.Set("X-Forwarded-Proto", tt.proto)
				}

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, req)
				assert.Equal(t, http.StatusNoContent, recorder.Code)
			}

			if !tt.wantWarn {
				assert.Empty(t, buf.String())
				return
			}

			assert.Equal(t, 1, strings.Count(buf.String(), "session.secure is false"))
			assert.Contains(t, buf.String(), `"level":"WARN"`)
		})
	}
}
