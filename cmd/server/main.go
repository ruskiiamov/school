package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"github.com/ruskiiamov/school/internal/app"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", configPathDefault(), "path to the YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	log, logFile, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close log file:", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)

	application, err := app.New(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		if err := application.Close(); err != nil {
			log.Error("close application", slog.Any("error", err))
		}
	}()

	return application.Run(ctx)
}

func configPathDefault() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}

	return config.DefaultPath
}
