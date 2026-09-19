package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPath       = "config.yaml"
	adminExampleValue = "change-me"
)

type Config struct {
	School   School  `yaml:"school"`
	Timezone string  `yaml:"timezone"`
	HTTP     HTTP    `yaml:"http"`
	DB       DB      `yaml:"db"`
	Log      Log     `yaml:"log"`
	Session  Session `yaml:"session"`
	Journal  Journal `yaml:"journal"`
	Files    Files   `yaml:"files"`
	Admin    Admin   `yaml:"admin"`

	Location *time.Location `yaml:"-"`
}

type School struct {
	Name           string     `yaml:"name"`
	YearStartMonth time.Month `yaml:"year_start_month"`
}

type HTTP struct {
	Addr            string        `yaml:"addr"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type DB struct {
	Path string `yaml:"path"`
}

type Log struct {
	File       string `yaml:"file"`
	Level      Level  `yaml:"level"`
	Stdout     bool   `yaml:"stdout"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

type Session struct {
	CookieName      string        `yaml:"cookie_name"`
	TTL             time.Duration `yaml:"ttl"`
	Secure          bool          `yaml:"secure"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

type Journal struct {
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

type Files struct {
	Dir             string        `yaml:"dir"`
	MaxFileSizeMB   int           `yaml:"max_file_size_mb"`
	MaxPerLesson    int           `yaml:"max_per_lesson"`
	TransferTimeout time.Duration `yaml:"transfer_timeout"`
}

func (f Files) MaxFileSize() int64 {
	return int64(f.MaxFileSizeMB) << 20
}

type Admin struct {
	Login      string `yaml:"login"`
	Password   string `yaml:"password"`
	LastName   string `yaml:"last_name"`
	FirstName  string `yaml:"first_name"`
	MiddleName string `yaml:"middle_name"`
}

type Level slog.Level

func (l *Level) UnmarshalYAML(node *yaml.Node) error {
	var name string
	if err := node.Decode(&name); err != nil {
		return err
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return fmt.Errorf("unknown log level %q", name)
	}

	*l = Level(level)

	return nil
}

func (l Level) Slog() slog.Level {
	return slog.Level(l)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Config{
		School:   School{Name: "Школа", YearStartMonth: time.August},
		Timezone: "Europe/Moscow",
		HTTP:     HTTP{Addr: ":8080", ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: time.Minute, ShutdownTimeout: 10 * time.Second},
		Log:      Log{Level: Level(slog.LevelInfo), MaxSizeMB: 10, MaxBackups: 5, MaxAgeDays: 30},
		Session:  Session{CookieName: "sid", TTL: 12 * time.Hour, CleanupInterval: time.Hour},
		Journal:  Journal{CleanupInterval: 24 * time.Hour},
		Files:    Files{MaxFileSizeMB: 10, MaxPerLesson: 10, TransferTimeout: 5 * time.Minute},
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("resolve config directory: %w", err)
	}

	cfg.DB.Path = resolvePath(base, cfg.DB.Path)
	cfg.Log.File = resolvePath(base, cfg.Log.File)
	cfg.Files.Dir = resolvePath(base, cfg.Files.Dir)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func resolvePath(base, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(base, path)
}

func (c *Config) validate() error {
	var problems []error

	require := func(ok bool, msg string) {
		if !ok {
			problems = append(problems, errors.New(msg))
		}
	}

	require(c.School.Name != "", "school.name is empty")
	require(c.School.YearStartMonth >= time.January && c.School.YearStartMonth <= time.December,
		"school.year_start_month must be between 1 and 12")
	require(c.Timezone != "", "timezone is empty")

	location, err := time.LoadLocation(c.Timezone)
	require(err == nil, fmt.Sprintf("timezone %q is unknown", c.Timezone))
	c.Location = location

	require(c.HTTP.Addr != "", "http.addr is empty")
	require(c.HTTP.ReadTimeout > 0, "http.read_timeout must be positive")
	require(c.HTTP.WriteTimeout > 0, "http.write_timeout must be positive")
	require(c.HTTP.IdleTimeout > 0, "http.idle_timeout must be positive")
	require(c.HTTP.ShutdownTimeout > 0, "http.shutdown_timeout must be positive")
	require(c.DB.Path != "", "db.path is empty")
	require(c.Log.File != "", "log.file is empty")
	require(c.Log.MaxSizeMB > 0, "log.max_size_mb must be positive")
	require(c.Log.MaxBackups >= 0, "log.max_backups must not be negative")
	require(c.Log.MaxAgeDays >= 0, "log.max_age_days must not be negative")
	require(c.Session.CookieName != "", "session.cookie_name is empty")
	require(c.Session.TTL > 0, "session.ttl must be positive")
	require(c.Session.CleanupInterval > 0, "session.cleanup_interval must be positive")
	require(c.Journal.CleanupInterval > 0, "journal.cleanup_interval must be positive")
	require(c.Files.Dir != "", "files.dir is empty")
	require(c.Files.MaxFileSizeMB > 0, "files.max_file_size_mb must be positive")
	require(c.Files.MaxPerLesson > 0, "files.max_per_lesson must be positive")
	require(c.Files.TransferTimeout > 0, "files.transfer_timeout must be positive")
	require(c.Admin.Login != "", "admin.login is empty")
	require(c.Admin.Password != "", "admin.password is empty")
	require(c.Admin.Password != adminExampleValue, "admin.password is the example value, set your own")
	require(c.Admin.LastName != "", "admin.last_name is empty")
	require(c.Admin.FirstName != "", "admin.first_name is empty")

	return errors.Join(problems...)
}
