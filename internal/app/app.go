package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server"
	"github.com/ruskiiamov/school/internal/storage"
)

type App struct {
	log     *slog.Logger
	db      *sql.DB
	auth    *auth.Service
	journal *journal.Service
	http    *http.Server

	cleanupInterval        time.Duration
	journalCleanupInterval time.Duration
	shutdownTimeout        time.Duration
}

func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	db, err := storage.Open(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	if err := storage.Migrate(ctx, db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database: %w", closeErr))
		}

		return nil, err
	}

	authService := auth.NewService(
		storage.NewUserRepo(db),
		storage.NewSessionRepo(db),
		cfg.Session.TTL,
		log,
	)

	if err := authService.EnsureAdmin(ctx, cfg.Admin); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database: %w", closeErr))
		}

		return nil, err
	}

	schoolService := school.NewService(
		cfg.School.YearStartMonth,
		cfg.Location,
		storage.NewSubjectRepo(db),
		storage.NewWorkTypeRepo(db),
		storage.NewClassRepo(db),
		storage.NewClassStudentRepo(db),
		storage.NewParentChildRepo(db),
		storage.NewAssignmentRepo(db),
		storage.NewSubstitutionRepo(db),
		log,
	)

	journalService := journal.NewService(storage.NewLessonRepo(db), storage.NewMarkRepo(db), storage.NewLessonStudentRepo(db), storage.NewAssignmentRepo(db),
		storage.NewSubstitutionRepo(db), storage.NewClassRepo(db), storage.NewClassStudentRepo(db),
		storage.NewSubjectRepo(db), storage.NewWorkTypeRepo(db), log)

	return &App{
		log:     log,
		db:      db,
		auth:    authService,
		journal: journalService,
		http: &http.Server{
			Addr:         cfg.HTTP.Addr,
			Handler:      server.New(cfg, authService, schoolService, journalService, log).Handler(),
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
			IdleTimeout:  cfg.HTTP.IdleTimeout,
		},
		cleanupInterval:        cfg.Session.CleanupInterval,
		journalCleanupInterval: cfg.Journal.CleanupInterval,
		shutdownTimeout:        cfg.HTTP.ShutdownTimeout,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		a.log.Info("server started", slog.String("addr", a.http.Addr))

		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen and serve: %w", err)
		}

		return nil
	})

	group.Go(func() error {
		a.auth.RunSessionCleanup(groupCtx, a.cleanupInterval)

		return nil
	})

	group.Go(func() error {
		a.journal.RunCleanup(groupCtx, a.journalCleanupInterval)

		return nil
	})

	group.Go(func() error {
		<-groupCtx.Done()

		return a.shutdown()
	})

	return group.Wait()
}

func (a *App) Close() error {
	if err := a.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}

func (a *App) shutdown() error {
	a.log.Info("server is shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	if err := a.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	a.log.Info("server stopped")

	return nil
}
