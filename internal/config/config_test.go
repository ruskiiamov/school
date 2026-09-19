package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

func TestLoadExampleRefusesExamplePassword(t *testing.T) {
	t.Parallel()

	_, err := Load("../../config.example.yaml")
	require.Error(t, err)
	assert.Equal(t, "invalid config: admin.password is the example value, set your own", err.Error())
}

func TestLoadExample(t *testing.T) {
	t.Parallel()

	example, err := os.ReadFile("../../config.example.yaml")
	require.NoError(t, err)

	path := writeConfig(t, strings.Replace(string(example), "change-me", "own-secret", 1))
	dir := filepath.Dir(path)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.HTTP.Addr)
	assert.Equal(t, 5*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, 12*time.Hour, cfg.Session.TTL)
	assert.Equal(t, time.Hour, cfg.Session.CleanupInterval)
	assert.Equal(t, 24*time.Hour, cfg.Journal.CleanupInterval)
	assert.Equal(t, filepath.Join(dir, "data", "school.db"), cfg.DB.Path)
	assert.Equal(t, filepath.Join(dir, "logs", "app.log"), cfg.Log.File)
	assert.Equal(t, filepath.Join(dir, "data", "files"), cfg.Files.Dir)
	assert.Equal(t, int64(10<<20), cfg.Files.MaxFileSize())
	assert.Equal(t, 10, cfg.Files.MaxPerLesson)
	assert.Equal(t, 5*time.Minute, cfg.Files.TransferTimeout)
	assert.Equal(t, slog.LevelInfo, cfg.Log.Level.Slog())
	assert.True(t, cfg.Log.Stdout)
	assert.Equal(t, "admin", cfg.Admin.Login)
	assert.Equal(t, "Europe/Moscow", cfg.Timezone)
	assert.Equal(t, "Europe/Moscow", cfg.Location.String())
	assert.Equal(t, time.August, cfg.School.YearStartMonth)
}

func TestLoadKeepsAbsolutePaths(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "elsewhere", "app.log")
	cfg, err := Load(writeConfig(t, "db:\n  path: ./x.db\nlog:\n  file: "+logFile+"\nfiles:\n  dir: ./x\nadmin:\n  login: a\n  password: p\n  full_name: n\n"))
	require.NoError(t, err)

	assert.Equal(t, logFile, cfg.Log.File)
	assert.True(t, filepath.IsAbs(cfg.DB.Path))
	assert.Equal(t, "x.db", filepath.Base(cfg.DB.Path))
}

func TestLoadYearStartMonth(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeConfig(t, "school:\n  year_start_month: 9\ndb:\n  path: ./x.db\nlog:\n  file: ./x.log\nfiles:\n  dir: ./x\nadmin:\n  login: a\n  password: p\n  full_name: n\n"))
	require.NoError(t, err)

	assert.Equal(t, time.September, cfg.School.YearStartMonth)
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
			name: "unknown timezone",
			body: "timezone: Mars/Olympus\n",
			want: `timezone "Mars/Olympus" is unknown`,
		},
		{
			name: "year start month out of range",
			body: "school:\n  year_start_month: 13\n",
			want: "school.year_start_month must be between 1 and 12",
		},
		{
			name: "zero year start month",
			body: "school:\n  year_start_month: 0\n",
			want: "school.year_start_month must be between 1 and 12",
		},
		{
			name: "missing files dir",
			body: "db:\n  path: ./x.db\nlog:\n  file: ./x.log\nadmin:\n  login: a\n  password: p\n  full_name: n\n",
			want: "files.dir is empty",
		},
		{
			name: "zero max file size",
			body: "files:\n  max_file_size_mb: 0\n",
			want: "files.max_file_size_mb must be positive",
		},
		{
			name: "example admin password",
			body: "db:\n  path: ./x.db\nlog:\n  file: ./x.log\nfiles:\n  dir: ./x\nadmin:\n  login: a\n  password: change-me\n  full_name: n\n",
			want: "admin.password is the example value, set your own",
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
