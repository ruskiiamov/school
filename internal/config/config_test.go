package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func TestLoadExample(t *testing.T) {
	t.Parallel()

	cfg, err := Load("../../config.example.yaml")
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.HTTP.Addr)
	assert.Equal(t, 5*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, 12*time.Hour, cfg.Session.TTL)
	assert.Equal(t, time.Hour, cfg.Session.CleanupInterval)
	assert.Equal(t, slog.LevelInfo, cfg.Log.Level.Slog())
	assert.True(t, cfg.Log.Stdout)
	assert.Equal(t, "admin", cfg.Admin.Login)
}

func TestLoadInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing admin login",
			body: "db:\n  path: ./x.db\nlog:\n  file: ./x.log\nadmin:\n  password: p\n  full_name: n\n",
			want: "admin.login is empty",
		},
		{
			name: "zero session ttl",
			body: "db:\n  path: ./x.db\nlog:\n  file: ./x.log\nsession:\n  ttl: 0s\nadmin:\n  login: a\n  password: p\n  full_name: n\n",
			want: "session.ttl must be positive",
		},
		{
			name: "unknown log level",
			body: "log:\n  level: loud\n",
			want: `unknown log level "loud"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(writeConfig(t, tt.body))
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	assert.ErrorContains(t, err, "read config")
}
