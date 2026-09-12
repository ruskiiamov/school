package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/ruskiiamov/school/internal/config"
)

func New(cfg config.Log) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.File), 0o750); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}

	file := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
	}

	var out io.Writer = file
	if cfg.Stdout {
		out = io.MultiWriter(file, os.Stdout)
	}

	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: cfg.Level.Slog()})
	log := slog.New(handler)
	slog.SetDefault(log)

	return log, file, nil
}
